package exec

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/suite"
)

type scannerSuite struct {
	suite.Suite
}

func TestScanner(t *testing.T) {
	suite.Run(t, &scannerSuite{})
}

func (s *scannerSuite) TestScanStructs() {
	type StructWithTags struct {
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	db, mock, err := sqlmock.New()
	s.Require().NoError(err)

	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			AddRow(testAddr1, testName1).
			AddRow(testAddr2, testName2),
		)
	rows, err := db.QueryContext(context.Background(), `SELECT * FROM "items"`)
	s.Require().NoError(err)

	sc := NewScanner(rows, "db")

	result := make([]StructWithTags, 0)
	err = sc.ScanStructs(result)
	s.Require().EqualError(err, errScanSliceTarget(result).Error())

	err = sc.ScanStructs(&result)
	s.Require().NoError(err)
	s.Require().ElementsMatch(
		[]StructWithTags{{Address: testAddr1, Name: testName1}, {Address: testAddr2, Name: testName2}},
		result,
	)
}

func (s *scannerSuite) TestScanVals() {
	db, mock, err := sqlmock.New()
	s.Require().NoError(err)

	mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	rows, err := db.QueryContext(context.Background(), `SELECT "id" FROM "items"`)
	s.Require().NoError(err)

	sc := NewScanner(rows, "db")

	result := make([]int, 0)
	err = sc.ScanVals(result)
	s.Require().EqualError(err, errScanSliceTarget(result).Error())

	err = NewScanner(rows, "json").ScanVals(&[]struct {
		ID int64 `json:"id"`
	}{})
	s.Require().ErrorContains(err, "is a row struct; use ScanStructs")

	err = sc.ScanVals(&[][]int64{})
	s.Require().EqualError(err, "goqu: ScanVals cannot scan into []int64; use []byte or a concrete sql.Scanner")
	s.True(errors.IsUnsupportedScanType(err))

	err = sc.ScanVals(&result)
	s.Require().NoError(err)
	s.Require().ElementsMatch([]int{1, 2}, result)
}

func (s *scannerSuite) TestScanVal_rejectsNilAndNonPointerTargets() {
	db, mock, err := sqlmock.New()
	s.Require().NoError(err)
	mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	rows, err := db.QueryContext(context.Background(), `SELECT "id" FROM "items"`)
	s.Require().NoError(err)

	sc := NewScanner(rows, "db")
	s.Require().True(sc.Next())
	for target, want := range map[interface{}]string{
		nil:           "goqu: scan target must not be nil, got <nil>",
		(*int64)(nil): "goqu: scan target must not be nil, got *int64",
		int64(1):      "goqu: cannot scan into int64, a pointer is required",
	} {
		err = sc.ScanVal(target)
		s.EqualError(err, want)
		s.True(errors.IsUnsupportedScanType(err))
	}
	var id int64
	s.NoError(sc.ScanVal(&id))
	s.Equal(int64(1), id)
	s.NoError(sc.Close())
}

func (s *scannerSuite) TestScanStruct_rejectsTargetsThatAreNotRows() {
	db, mock, err := sqlmock.New()
	s.Require().NoError(err)
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"string", "valid"}).AddRow("a", true))
	rows, err := db.QueryContext(context.Background(), `SELECT * FROM "items"`)
	s.Require().NoError(err)

	sc := NewScanner(rows, "db")
	s.Require().True(sc.Next())
	err = sc.ScanStruct(&sql.NullString{})
	s.EqualError(err, "goqu: cannot scan rows into *sql.NullString, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[sql.NullString]/ScanOneVal[sql.NullString] or IterVals[sql.NullString]")
	s.True(errors.IsUnsupportedScanType(err))

	type row struct {
		Name string `db:"string"`
	}
	err = sc.ScanStruct((*row)(nil))
	s.EqualError(err, "goqu: scan target must not be nil, got *exec.row")
	s.True(errors.IsUnsupportedScanType(err))
	err = sc.ScanStruct(row{})
	s.EqualError(err, "goqu: cannot scan into exec.row, a pointer to a struct is required")
	s.True(errors.IsUnsupportedScanType(err))

	type node struct {
		Name string `db:"string"`
		Next *node
	}
	err = sc.ScanStruct(&node{})
	s.EqualError(err, "goqu: cannot scan rows into *exec.node: field Next leads back to exec.node; tag it `db:\"-\"`")
	s.True(errors.IsUnsupportedScanType(err))
	s.NoError(sc.Close())
}

type (
	boolRefRef         *boolRef
	nestedNamedPointer struct {
		Flag  *boolRef          `db:"flag"`
		Chain boolRefRef        `db:"chain"`
		Null  sql.Null[boolRef] `db:"null"`
	}
	rawBytesRow struct {
		Data sql.RawBytes `db:"data"`
	}
)

func (s *scannerSuite) scanner(columns []string, rows ...[]driver.Value) Scanner {
	db, mock, err := sqlmock.New()
	s.Require().NoError(err)
	mockRows := sqlmock.NewRows(columns)
	for _, row := range rows {
		mockRows.AddRow(row...)
	}
	mock.ExpectQuery(`SELECT`).WillReturnRows(mockRows)
	result, err := db.QueryContext(context.Background(), `SELECT`)
	s.Require().NoError(err)
	sc := NewScanner(result, "db")
	s.T().Cleanup(func() {
		s.NoError(sc.Close())
		s.NoError(mock.ExpectationsWereMet())
	})
	return sc
}

func (s *scannerSuite) TestScanVal_fillsNamedPointersAtAnyDepth() {
	sc := s.scanner([]string{"v"}, []driver.Value{int64(1)}, []driver.Value{nil})
	var (
		flag  bool
		ptr   *boolRef
		chain boolRefRef
		null  sql.Null[boolRef]
	)
	s.Require().True(sc.Next())
	s.Require().NoError(sc.ScanVal(boolRef(&flag)))
	s.Require().NoError(sc.ScanVal(&ptr))
	s.Require().NoError(sc.ScanVal(&chain))
	s.Require().NoError(sc.ScanVal(&null))
	s.True(flag)
	s.True(**ptr)
	s.True(**chain)
	s.True(null.Valid)
	s.True(*null.V)

	s.Require().True(sc.Next())
	s.Require().NoError(sc.ScanVal(&ptr))
	s.Require().NoError(sc.ScanVal(&chain))
	s.Require().NoError(sc.ScanVal(&null))
	s.Nil(ptr)
	s.Nil(chain)
	s.Equal(sql.Null[boolRef]{}, null)

	var flags []*boolRef
	s.Require().NoError(s.scanner([]string{"v"}, []driver.Value{int64(1)}, []driver.Value{nil}).ScanVals(&flags))
	s.Require().Len(flags, 2)
	s.True(**flags[0])
	s.Nil(flags[1])
}

func (s *scannerSuite) TestScanStructs_fillNamedPointerFieldsAtAnyDepth() {
	sc := s.scanner([]string{"flag", "chain", "null"}, []driver.Value{int64(1), int64(1), int64(1)}, []driver.Value{nil, nil, nil})
	var rows []nestedNamedPointer
	s.Require().NoError(sc.ScanStructs(&rows))
	s.Require().Len(rows, 2)
	s.True(**rows[0].Flag)
	s.True(**rows[0].Chain)
	s.True(rows[0].Null.Valid)
	s.True(*rows[0].Null.V)
	s.Equal(nestedNamedPointer{}, rows[1])
}

func (s *scannerSuite) TestRawBytesFields_scanOneRowAtATimeOnly() {
	sc := s.scanner([]string{"data"}, []driver.Value{[]byte("a")})
	s.Require().True(sc.Next())
	var row rawBytesRow
	s.Require().NoError(sc.ScanStruct(&row))
	s.Equal(sql.RawBytes("a"), row.Data)

	err := s.scanner([]string{"data"}, []driver.Value{[]byte("a")}).ScanStructs(&[]rawBytesRow{})
	s.EqualError(err, "goqu: cannot scan rows into exec.rawBytesRow: field Data holds a sql.RawBytes, "+
		"which is only valid until the next row is read or the rows are closed; use []byte or tag it `db:\"-\"`")
	s.True(errors.IsUnsupportedScanType(err))
}
