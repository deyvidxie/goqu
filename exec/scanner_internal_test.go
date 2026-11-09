package exec

import (
	"context"
	"database/sql"
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
	s.NoError(sc.Close())
}
