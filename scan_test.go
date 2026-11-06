package goqu_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exec"
	"github.com/stretchr/testify/suite"
)

type (
	scanSuite struct {
		suite.Suite
		db   *goqu.Database
		mock sqlmock.Sqlmock
	}
	scanTestError          struct{}
	scanTestItemRef        *dsTestActionItem
	scanTestNullStringRef  *sql.NullString
	scanTestTags           []string
	scanTestScannerRow     dsTestActionItem
	scanTestEmbedded       struct{ time.Time }
	scanTestEmbeddedPtr    struct{ *time.Time }
	ScanTestItem           dsTestActionItem
	scanTestEmbeddedRow    struct{ ScanTestItem }
	scanTestEmbeddedRowPtr struct{ *ScanTestItem }
	scanTestUnexportedRow  struct{ dsTestActionItem }
	scanTestUnexportedPtr  struct{ *dsTestActionItem }
	scanTestOpaqueScanner  struct{}
	ScanTestID             int
	ScanTestName           string
	scanTestEmbeddedID     struct {
		*ScanTestID
		Name string `db:"name"`
	}
	scanTestNamed   struct{ ScanTestName }
	scanTestTimeRow struct {
		Created time.Time `db:"created"`
		Note    *string   `db:"note"`
	}
	scanTestBlob        []byte
	scanTestNestedID    struct{ scanTestEmbeddedID }
	scanTestNestedIDPtr struct{ *scanTestEmbeddedID }
	scanTestMixed       struct {
		strings.Builder
		Name string `db:"name"`
	}
	ScanTestSelfRef struct{ *ScanTestSelfRef }
	scanTestTagged  struct {
		ID int64 `json:"id"`
	}
)

func (t *scanTestTags) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return scanTestError{}
	}
	*t = strings.Split(string(b), ",")
	return nil
}

func (*scanTestScannerRow) Scan(interface{}) error {
	return scanTestError{}
}

func (scanTestError) Error() string { return "scan failed" }

func (*scanTestOpaqueScanner) Scan(interface{}) error {
	return scanTestError{}
}

func (ss *scanSuite) SetupTest() {
	mDB, mock, err := sqlmock.New()
	ss.Require().NoError(err)
	ss.db = goqu.New("mock", mDB)
	ss.mock = mock
}

func (ss *scanSuite) TearDownTest() {
	ss.NoError(ss.mock.ExpectationsWereMet())
}

func (ss *scanSuite) itemRows(csv string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"address", "name"}).FromCSVString(csv)
}

func (ss *scanSuite) TestScanAllStructs() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2"))

	items, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestScanAllStructsPointers() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	items, err := goqu.ScanAllStructs[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]*dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructsPrepared() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" WHERE \("name" = \?\)`).
		WithArgs("Test1").
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.From("items").Prepared(true).Where(goqu.C("name").Eq("Test1"))
	items, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), ds)
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructsNoRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows(""))

	items, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(scanTestError{})

	items, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsRowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2").RowError(1, scanTestError{}))

	items, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsFromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" \("address", "name"\) VALUES \('111 Test Addr', 'Test1'\) RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), ds.Executor())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructsRequiresStruct() {
	items, err := goqu.ScanAllStructs[string](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs requires a struct or a pointer to a struct, got string")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsWithoutDatabase() {
	_, err := goqu.ScanAllStructs[dsTestActionItem](context.Background(), goqu.From("items"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanOneStruct() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOneStruct[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStructPointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOneStruct[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStructPointerNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := goqu.ScanOneStruct[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneStructNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := goqu.ScanOneStruct[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneStructQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := goqu.ScanOneStruct[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneStructRowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1").RowError(0, scanTestError{}))

	item, found, err := goqu.ScanOneStruct[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneStructFromExecutor() {
	ss.mock.ExpectQuery(`UPDATE "items" SET "name"='Test2' RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test2"))

	ds := ss.db.Update("items").Set(goqu.Record{"name": "Test2"}).Returning("address", "name")
	item, found, err := goqu.ScanOneStruct[*dsTestActionItem](context.Background(), ds.Executor())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test2"}, item)
}

func (ss *scanSuite) TestScanOneStructRequiresStruct() {
	_, found, err := goqu.ScanOneStruct[string](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct requires a struct or a pointer to a struct, got string")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.False(found)

	row, found, err := goqu.ScanOneStruct[**dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct requires a struct or a pointer to a struct, got **goqu_test.dsTestActionItem")
	ss.False(found)
	ss.Nil(row)
}

func (ss *scanSuite) TestScanRejectsSingleColumnStructs() {
	_, err := goqu.ScanAllStructs[time.Time](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs requires a row struct, got time.Time, which scans as one column; use ScanAllVals[time.Time]")
	ss.True(exec.IsUnsupportedScanType(err))

	_, err = goqu.ScanAllStructs[*time.Time](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs requires a row struct, got *time.Time, which scans as one column; use ScanAllVals[time.Time]")

	_, _, err = goqu.ScanOneStruct[sql.NullString](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct requires a row struct, got sql.NullString, which scans as one column; "+
		"use ScanOneVal[sql.NullString]")

	_, _, err = goqu.ScanOneStruct[*sql.NullString](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct requires a row struct, got *sql.NullString, which scans as one column; "+
		"use ScanOneVal[sql.NullString]")

	_, err = goqu.ScanAllStructs[scanTestOpaqueScanner](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs requires a row struct, got goqu_test.scanTestOpaqueScanner, which scans as one column; "+
		"use ScanAllVals[goqu_test.scanTestOpaqueScanner]")
}

func (ss *scanSuite) TestScanRejectsStructsWithoutExportedFields() {
	for name, scan := range map[string]func() error{
		"netip.Addr": func() error {
			_, err := goqu.ScanAllStructs[netip.Addr](context.Background(), ss.db.From("items"))
			return err
		},
		"goqu_test.scanTestEmbedded": func() error {
			_, err := goqu.ScanAllStructs[scanTestEmbedded](context.Background(), ss.db.From("items"))
			return err
		},
		"goqu_test.scanTestEmbeddedPtr": func() error {
			_, err := goqu.ScanAllStructs[scanTestEmbeddedPtr](context.Background(), ss.db.From("items"))
			return err
		},
		"goqu_test.ScanTestSelfRef": func() error {
			_, err := goqu.ScanAllStructs[ScanTestSelfRef](context.Background(), ss.db.From("items"))
			return err
		},
		"goqu_test.scanTestUnexportedPtr": func() error {
			_, err := goqu.ScanAllStructs[scanTestUnexportedPtr](context.Background(), ss.db.From("items"))
			return err
		},
		"strings.Builder": func() error {
			_, err := goqu.ScanAllStructs[strings.Builder](context.Background(), ss.db.From("items"))
			return err
		},
	} {
		err := scan()
		ss.EqualError(err, "goqu: ScanAllStructs requires a row struct, got "+name+", which has no fields goqu can scan into", name)
		ss.True(exec.IsUnsupportedScanType(err), name)
	}
}

func (ss *scanSuite) TestScanAllStructsAcceptsEmbeddedRowFields() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("211 Test Addr,Test2"))
	ss.mock.ExpectQuery(`SELECT "name" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Test3"))
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("411 Test Addr,Test4"))

	embedded, err := goqu.ScanAllStructs[scanTestEmbeddedRow](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]scanTestEmbeddedRow{{ScanTestItem{Address: "111 Test Addr", Name: "Test1"}}}, embedded)

	embeddedPtr, err := goqu.ScanAllStructs[scanTestEmbeddedRowPtr](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]scanTestEmbeddedRowPtr{{&ScanTestItem{Address: "211 Test Addr", Name: "Test2"}}}, embeddedPtr)

	mixed, err := goqu.ScanAllStructs[scanTestMixed](context.Background(), ss.db.From("items").Select("name"))
	ss.NoError(err)
	ss.Equal([]scanTestMixed{{Name: "Test3"}}, mixed)

	unexported, err := goqu.ScanAllStructs[scanTestUnexportedRow](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]scanTestUnexportedRow{{dsTestActionItem{Address: "411 Test Addr", Name: "Test4"}}}, unexported)
}

func (ss *scanSuite) TestScanAllStructsTypeCheckIgnoresTagName() {
	goqu.SetIgnoreUntaggedFields(true)
	defer goqu.SetIgnoreUntaggedFields(false)
	mDB, mock, err := sqlmock.New()
	ss.Require().NoError(err)
	mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

	rows, err := goqu.ScanAllStructs[scanTestTagged](context.Background(), goqu.NewWithTagName("mock", "json", mDB).From("items"))
	ss.NoError(err)
	ss.Equal([]scanTestTagged{{ID: 7}}, rows)
	ss.NoError(mock.ExpectationsWereMet())
}

func (ss *scanSuite) TestScanOneStructAcceptsRowStructWithScan() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOneStruct[scanTestScannerRow](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestScannerRow{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStructNamedPointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOneStruct[scanTestItemRef](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestItemRef(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}), item)
}

func (ss *scanSuite) TestScanOneStructPointerQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	item, found, err := goqu.ScanOneStruct[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneStructWithoutDatabase() {
	_, _, err := goqu.ScanOneStruct[dsTestActionItem](context.Background(), goqu.From("items"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanAllVals() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ids, err := goqu.ScanAllVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanAllValsNoRows() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := goqu.ScanAllVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllValsQueryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnError(scanTestError{})

	ids, err := goqu.ScanAllVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllValsRowError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2").RowError(1, scanTestError{}))

	ids, err := goqu.ScanAllVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllValsFromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" \("name"\) VALUES \('Test1'\), \('Test2'\) RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ds := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}, goqu.Record{"name": "Test2"}).Returning("id")
	ids, err := goqu.ScanAllVals[int64](context.Background(), ds.Executor())
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanAllValsWithoutDatabase() {
	_, err := goqu.ScanAllVals[int64](context.Background(), goqu.From("items").Select("id"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanOneVal() {
	ss.mock.ExpectQuery(`SELECT COUNT\(\*\) AS "count" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("10"))

	count, found, err := goqu.ScanOneVal[int64](context.Background(), ss.db.From("items").Select(goqu.COUNT("*").As("count")))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(10), count)
}

func (ss *scanSuite) TestScanOneValNotFound() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	id, found, err := goqu.ScanOneVal[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanOneValQueryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := goqu.ScanOneVal[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneValRowError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1").RowError(0, scanTestError{}))

	id, found, err := goqu.ScanOneVal[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanOneValFromExecutor() {
	ss.mock.ExpectQuery(`DELETE FROM "items" RETURNING "id"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("7"))

	id, found, err := goqu.ScanOneVal[int64](context.Background(), ss.db.Delete("items").Returning("id").Executor())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(7), id)
}

func (ss *scanSuite) TestScanOneValRejectsSlices() {
	_, _, err := goqu.ScanOneVal[[]int64](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneVal cannot scan into slice type []int64; use []byte or a sql.Scanner")
	ss.True(exec.IsUnsupportedScanType(err))

	_, _, err = goqu.ScanOneVal[*[]int64](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneVal cannot scan into slice type *[]int64; use []byte or a sql.Scanner")
	ss.True(exec.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestScanValsRejectRawBytes() {
	_, _, err := goqu.ScanOneVal[sql.RawBytes](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneVal cannot return sql.RawBytes, which is only valid until the next row; use []byte")
	ss.True(exec.IsUnsupportedScanType(err))

	_, err = goqu.ScanAllVals[sql.RawBytes](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot return sql.RawBytes, which is only valid until the next row; use []byte")
	ss.True(exec.IsUnsupportedScanType(err))

	_, _, err = goqu.ScanOneVal[*sql.RawBytes](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneVal cannot return sql.RawBytes, which is only valid until the next row; use []byte")

	_, err = goqu.ScanAllVals[*sql.RawBytes](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot return sql.RawBytes, which is only valid until the next row; use []byte")
}

func (ss *scanSuite) TestScanAllValsAcceptsNullablePointers() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")).AddRow(nil))
	data, err := goqu.ScanAllVals[*[]byte](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.Require().Len(data, 2)
	ss.Equal([]byte("abc"), *data[0])
	ss.Nil(*data[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	anys, err := goqu.ScanAllVals[*any](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.Require().Len(anys, 2)
	ss.Equal("abc", *anys[0])
	ss.Nil(*anys[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	named, err := goqu.ScanAllVals[scanTestNullStringRef](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.Equal([]scanTestNullStringRef{{String: "abc", Valid: true}, {}}, named)

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	nullStrings, err := goqu.ScanAllVals[*sql.NullString](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.Equal([]*sql.NullString{{String: "abc", Valid: true}, {}}, nullStrings)

	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)).AddRow(nil))
	ids, err := goqu.ScanAllVals[**int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.Require().Len(ids, 2)
	ss.Equal(int64(3), **ids[0])
	ss.Nil(*ids[1])
}

func (ss *scanSuite) TestScanAllValsRejectsNonNullablePointers() {
	ids, err := goqu.ScanAllVals[*int64](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot scan NULL into *int64; use sql.Null[int64]")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.Nil(ids)

	_, err = goqu.ScanAllVals[*fmt.Stringer](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into *fmt.Stringer")
	_, err = goqu.ScanAllVals[*[]int64](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into *[]int64")
	ss.True(exec.IsUnsupportedScanType(err))
	_, err = goqu.ScanAllVals[*uintptr](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into *uintptr")

	for _, c := range []struct {
		scan func() error
		want string
	}{
		{func() error {
			_, err := goqu.ScanAllVals[*string](context.Background(), ss.db.From("items"))
			return err
		}, "string"},
		{func() error { _, err := goqu.ScanAllVals[*bool](context.Background(), ss.db.From("items")); return err }, "bool"},
		{func() error {
			_, err := goqu.ScanAllVals[*float64](context.Background(), ss.db.From("items"))
			return err
		}, "float64"},
		{func() error {
			_, err := goqu.ScanAllVals[*time.Time](context.Background(), ss.db.From("items"))
			return err
		}, "time.Time"},
		{
			func() error {
				_, err := goqu.ScanAllVals[*scanTestBlob](context.Background(), ss.db.From("items"))
				return err
			},
			"goqu_test.scanTestBlob",
		},
	} {
		ss.EqualError(c.scan(), "goqu: ScanAllVals cannot scan NULL into *"+c.want+"; use sql.Null["+c.want+"]")
	}
}

func (ss *scanSuite) TestScanAllStructsAcceptsStructFieldsAndNonStructEmbeds() {
	created := time.Unix(1700000000, 0).UTC()
	ss.mock.ExpectQuery(`SELECT "created", "note" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"created", "note"}).AddRow(created, nil))
	ss.mock.ExpectQuery(`SELECT "scantestname" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"scantestname"}).AddRow("a"))
	ss.mock.ExpectQuery(`SELECT "isolation", "readonly" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"isolation", "readonly"}).AddRow(int64(2), true))

	times, err := goqu.ScanAllStructs[scanTestTimeRow](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]scanTestTimeRow{{Created: created}}, times)

	named, err := goqu.ScanAllStructs[scanTestNamed](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]scanTestNamed{{ScanTestName: "a"}}, named)

	opts, err := goqu.ScanAllStructs[sql.TxOptions](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]sql.TxOptions{{Isolation: sql.LevelReadCommitted, ReadOnly: true}}, opts)
}

func (ss *scanSuite) TestScanStructsRejectEmbeddedNonStructPointers() {
	_, err := goqu.ScanAllStructs[scanTestEmbeddedID](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs cannot scan goqu_test.scanTestEmbeddedID, which embeds *goqu_test.ScanTestID")
	ss.True(exec.IsUnsupportedScanType(err))

	_, err = goqu.ScanAllStructs[scanTestNestedID](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs cannot scan goqu_test.scanTestNestedID, which embeds *goqu_test.ScanTestID")

	_, _, err = goqu.ScanOneStruct[scanTestNestedIDPtr](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct cannot scan goqu_test.scanTestNestedIDPtr, which embeds *goqu_test.ScanTestID")
}

func (ss *scanSuite) TestScanOneValAcceptsBytes() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")))
	ss.mock.ExpectQuery(`SELECT "data" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("def")))

	data, found, err := goqu.ScanOneVal[[]byte](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal([]byte("abc"), data)

	ptr, found, err := goqu.ScanOneVal[*[]byte](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal([]byte("def"), *ptr)
}

func (ss *scanSuite) TestScanAllValsAcceptsNamedByteSlicesAndScanners() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte(`{"a":1}`)))
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")))

	docs, err := goqu.ScanAllVals[json.RawMessage](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.Equal([]json.RawMessage{json.RawMessage(`{"a":1}`)}, docs)

	all, err := goqu.ScanAllVals[sql.NullString](context.Background(), ss.db.From("items").Select("data"))
	ss.NoError(err)
	ss.Equal([]sql.NullString{{String: "abc", Valid: true}}, all)
}

func (ss *scanSuite) TestScanOneValAcceptsSliceScanners() {
	ss.mock.ExpectQuery(`SELECT "tags" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"tags"}).AddRow([]byte("a,b")))

	tags, found, err := goqu.ScanOneVal[scanTestTags](context.Background(), ss.db.From("items").Select("tags"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTags{"a", "b"}, tags)

	ss.mock.ExpectQuery(`SELECT "tags" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"tags"}).AddRow([]byte("c")))
	tagsPtr, found, err := goqu.ScanOneVal[*scanTestTags](context.Background(), ss.db.From("items").Select("tags"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTags{"c"}, *tagsPtr)
}

func (ss *scanSuite) TestScanOneValAcceptsPointersToNamedByteSlices() {
	ss.mock.ExpectQuery(`SELECT "doc" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow([]byte(`{"a":1}`)))
	ss.mock.ExpectQuery(`SELECT "doc" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow(nil))

	doc, found, err := goqu.ScanOneVal[*json.RawMessage](context.Background(), ss.db.From("items").Select("doc"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(json.RawMessage(`{"a":1}`), *doc)

	doc, found, err = goqu.ScanOneVal[*json.RawMessage](context.Background(), ss.db.From("items").Select("doc"))
	ss.NoError(err)
	ss.True(found)
	ss.Nil(doc)
}

func (ss *scanSuite) TestScanOneValWithoutDatabase() {
	_, _, err := goqu.ScanOneVal[int64](context.Background(), goqu.From("items").Select("id"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(scanSuite))
}
