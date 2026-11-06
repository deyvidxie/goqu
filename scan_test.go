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
	scanTestScannedBlob []byte
	scanTestJSONSkip    struct {
		*ScanTestID `json:"-"`
		ID          int64 `db:"id"`
	}
	scanTestDBSkip struct {
		*ScanTestID `db:"-"`
		ID          int64 `json:"id"`
	}
	ScanTestAudit    struct{ *ScanTestID }
	scanTestAuditRow struct {
		ID    int64         `db:"id"`
		Audit ScanTestAudit `db:"audit"`
	}
	ScanTestPoint struct {
		X int64 `db:"x"`
	}
	scanTestSegment struct {
		From ScanTestPoint `db:"from"`
		To   ScanTestPoint `db:"to"`
	}
	ScanTestNode struct {
		ID     int64 `db:"id"`
		Parent *ScanTestNode
	}
	ScanTestTime      time.Time
	scanTestSkippedID struct {
		*ScanTestID `db:"-"`
		ID          int64 `db:"id"`
	}
	scanTestSkippedNested struct {
		scanTestEmbeddedID `db:"-"`
		ID                 int64 `db:"id"`
	}
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
	ScanTestTree struct {
		ID       int64         `db:"id"`
		Parent   *ScanTestTree `db:"-"`
		children *ScanTestTree
	}
	scanTestNodeRow struct {
		ID   int64        `db:"id"`
		Node ScanTestNode `db:"node"`
	}
	scanTestMeta struct {
		Note string `db:"-"`
	}
	scanTestHiddenMeta struct {
		*scanTestMeta
		ID int64 `db:"id"`
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

func (b *scanTestScannedBlob) Scan(src interface{}) error {
	v, ok := src.([]byte)
	if !ok {
		return scanTestError{}
	}
	*b = append(scanTestScannedBlob{}, v...)
	return nil
}

func allStructsErr[T any](ss *scanSuite) error {
	rows, err := ss.db.From("items").ScanAllStructs[T](context.Background())
	ss.Nil(rows)
	return err
}

func allValsErr[T any](ss *scanSuite) error {
	vals, err := ss.db.From("items").ScanAllVals[T](context.Background())
	ss.Nil(vals)
	return err
}

func oneValErr[T any](ss *scanSuite) error {
	_, found, err := ss.db.From("items").ScanOneVal[T](context.Background())
	ss.False(found)
	return err
}

func (*scanTestOpaqueScanner) Scan(interface{}) error {
	return scanTestError{}
}

func (ss *scanSuite) SetupTest() {
	mDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
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

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestScanAllStructsPointers() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	items, err := ss.db.From("items").ScanAllStructs[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]*dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructsPrepared() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" WHERE ("name" = ?)`).
		WithArgs("Test1").
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.From("items").Prepared(true).Where(goqu.C("name").Eq("Test1"))
	items, err := ds.ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructsNoRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows(""))

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(scanTestError{})

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsRowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2").RowError(1, scanTestError{}))

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructsFromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("address", "name") VALUES ('111 Test Addr', 'Test1') RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, err := ds.Executor().ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructsRequiresStruct() {
	items, err := ss.db.From("items").ScanAllStructs[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string: requires a struct or a pointer to a struct")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.Nil(items)
}

func (ss *scanSuite) TestExecutorScanStructsRequireStruct() {
	executor := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}).Returning("name").Executor()

	names, err := executor.ScanAllStructs[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string: requires a struct or a pointer to a struct")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.Nil(names)

	_, found, err := executor.ScanOneStruct[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string: requires a struct or a pointer to a struct")
	ss.False(found)
}

func (ss *scanSuite) TestScanAllStructsWithoutDatabase() {
	_, err := goqu.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanOneStruct() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStructPointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStructPointerNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := ss.db.From("items").ScanOneStruct[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneStructNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneStructQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneStructRowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1").RowError(0, scanTestError{}))

	item, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneStructFromExecutor() {
	ss.mock.ExpectQuery(`UPDATE "items" SET "name"='Test2' RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test2"))

	ds := ss.db.Update("items").Set(goqu.Record{"name": "Test2"}).Returning("address", "name")
	item, found, err := ds.Executor().ScanOneStruct[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test2"}, item)
}

func (ss *scanSuite) TestScanOneStructRequiresStruct() {
	_, found, err := ss.db.From("items").ScanOneStruct[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string: requires a struct or a pointer to a struct")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.False(found)

	row, found, err := ss.db.From("items").ScanOneStruct[**dsTestActionItem](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into **goqu_test.dsTestActionItem: requires a struct or a pointer to a struct")
	ss.False(found)
	ss.Nil(row)
}

func (ss *scanSuite) TestScanRejectsSingleColumnStructs() {
	_, err := ss.db.From("items").ScanAllStructs[time.Time](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into time.Time, which scans as one column; "+
		"use ScanAllVals[time.Time] or ScanOneVal[time.Time]")
	ss.True(exec.IsUnsupportedScanType(err))

	_, err = ss.db.From("items").ScanAllStructs[*time.Time](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into *time.Time, which scans as one column; "+
		"use ScanAllVals[time.Time] or ScanOneVal[time.Time]")

	_, _, err = ss.db.From("items").ScanOneStruct[sql.NullString](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into sql.NullString, which scans as one column; "+
		"use ScanAllVals[sql.NullString] or ScanOneVal[sql.NullString]")

	_, _, err = ss.db.From("items").ScanOneStruct[*sql.NullString](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into *sql.NullString, which scans as one column; "+
		"use ScanAllVals[sql.NullString] or ScanOneVal[sql.NullString]")

	_, err = ss.db.From("items").ScanAllStructs[scanTestOpaqueScanner](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.scanTestOpaqueScanner, which scans as one column; "+
		"use ScanAllVals[goqu_test.scanTestOpaqueScanner] or ScanOneVal[goqu_test.scanTestOpaqueScanner]")
}

func (ss *scanSuite) TestScanRejectsStructsWithoutRowFields() {
	for name, err := range map[string]error{
		"netip.Addr":                    allStructsErr[netip.Addr](ss),
		"goqu_test.scanTestEmbedded":    allStructsErr[scanTestEmbedded](ss),
		"goqu_test.scanTestEmbeddedPtr": allStructsErr[scanTestEmbeddedPtr](ss),
		"strings.Builder":               allStructsErr[strings.Builder](ss),
	} {
		ss.EqualError(err, "goqu: cannot scan rows into "+name+", which has no fields goqu can scan into", name)
		ss.True(exec.IsUnsupportedScanType(err), name)
	}

	ss.EqualError(allStructsErr[*strings.Builder](ss), "goqu: cannot scan rows into *strings.Builder, which has no fields goqu can scan into")
}

func (ss *scanSuite) TestScanAllStructsAcceptsEmbeddedRowFields() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("211 Test Addr,Test2"))
	ss.mock.ExpectQuery(`SELECT "name" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Test3"))
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("411 Test Addr,Test4"))

	embedded, err := ss.db.From("items").ScanAllStructs[scanTestEmbeddedRow](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestEmbeddedRow{{ScanTestItem{Address: "111 Test Addr", Name: "Test1"}}}, embedded)

	embeddedPtr, err := ss.db.From("items").ScanAllStructs[scanTestEmbeddedRowPtr](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestEmbeddedRowPtr{{&ScanTestItem{Address: "211 Test Addr", Name: "Test2"}}}, embeddedPtr)

	mixed, err := ss.db.From("items").Select("name").ScanAllStructs[scanTestMixed](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestMixed{{Name: "Test3"}}, mixed)

	unexported, err := ss.db.From("items").ScanAllStructs[scanTestUnexportedRow](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestUnexportedRow{{dsTestActionItem{Address: "411 Test Addr", Name: "Test4"}}}, unexported)
}

func (ss *scanSuite) TestScanStructsUseDatasetTagName() {
	goqu.SetIgnoreUntaggedFields(true)
	defer goqu.SetIgnoreUntaggedFields(false)
	mDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	ss.Require().NoError(err)
	mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(8)))
	db := goqu.NewWithTagName("mock", "json", mDB)

	rows, err := db.From("items").ScanAllStructs[scanTestTagged](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestTagged{{ID: 7}}, rows)

	row, found, err := db.From("items").ScanOneStruct[scanTestTagged](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTagged{ID: 8}, row)
	ss.NoError(mock.ExpectationsWereMet())

	_, err = db.From("items").ScanAllStructs[scanTestDBSkip](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.scanTestDBSkip: embedded field ScanTestID is a pointer to "+
		"non-struct goqu_test.ScanTestID; tag it `json:\"-\"`")
}

func (ss *scanSuite) TestScanOneStructAcceptsRowStructWithScan() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[scanTestScannerRow](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestScannerRow{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStructNamedPointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[scanTestItemRef](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestItemRef(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}), item)
}

func (ss *scanSuite) TestScanOneStructPointerQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	item, found, err := ss.db.From("items").ScanOneStruct[*dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneStructWithoutDatabase() {
	_, _, err := goqu.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanAllVals() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanAllValsNoRows() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.NoError(err)
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllValsQueryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnError(scanTestError{})

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllValsRowError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2").RowError(1, scanTestError{}))

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllValsFromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("name") VALUES ('Test1'), ('Test2') RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ds := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}, goqu.Record{"name": "Test2"}).Returning("id")
	ids, err := ds.Executor().ScanAllVals[int64](context.Background())
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanAllValsWithoutDatabase() {
	_, err := goqu.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanOneVal() {
	ss.mock.ExpectQuery(`SELECT COUNT(*) AS "count" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("10"))

	count, found, err := ss.db.From("items").Select(goqu.COUNT("*").As("count")).ScanOneVal[int64](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(10), count)
}

func (ss *scanSuite) TestScanOneValNotFound() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	id, found, err := ss.db.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.NoError(err)
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanOneValQueryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := ss.db.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneValRowError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1").RowError(0, scanTestError{}))

	id, found, err := ss.db.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanOneValFromExecutor() {
	ss.mock.ExpectQuery(`DELETE FROM "items" RETURNING "id"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("7"))

	id, found, err := ss.db.Delete("items").Returning("id").Executor().ScanOneVal[int64](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(7), id)
}

func (ss *scanSuite) TestScanOneValRejectsSlices() {
	for _, err := range []error{oneValErr[[]int64](ss), oneValErr[*[]int64](ss), oneValErr[**[]int64](ss)} {
		ss.EqualError(err, "goqu: ScanOneVal cannot scan into slice type []int64; use []byte or a sql.Scanner")
		ss.True(exec.IsUnsupportedScanType(err))
	}

	err := oneValErr[scanTestBlob](ss)
	ss.EqualError(err, "goqu: ScanOneVal cannot scan into goqu_test.scanTestBlob; use *goqu_test.scanTestBlob")
	ss.True(exec.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestScanValsRejectRawBytes() {
	for _, err := range []error{oneValErr[sql.RawBytes](ss), oneValErr[*sql.RawBytes](ss), oneValErr[**sql.RawBytes](ss)} {
		ss.EqualError(err, "goqu: ScanOneVal cannot return sql.RawBytes, which is only valid until the next row; use []byte")
		ss.True(exec.IsUnsupportedScanType(err))
	}
	for _, err := range []error{allValsErr[sql.RawBytes](ss), allValsErr[*sql.RawBytes](ss), allValsErr[**sql.RawBytes](ss)} {
		ss.EqualError(err, "goqu: ScanAllVals cannot return sql.RawBytes, which is only valid until the next row; use []byte")
		ss.True(exec.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestScanAllValsAcceptsNullablePointers() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")).AddRow(nil))
	data, err := ss.db.From("items").Select("data").ScanAllVals[*[]byte](context.Background())
	ss.NoError(err)
	ss.Require().Len(data, 2)
	ss.Equal([]byte("abc"), *data[0])
	ss.Nil(*data[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	anys, err := ss.db.From("items").Select("data").ScanAllVals[*any](context.Background())
	ss.NoError(err)
	ss.Require().Len(anys, 2)
	ss.Equal("abc", *anys[0])
	ss.Nil(*anys[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	named, err := ss.db.From("items").Select("data").ScanAllVals[scanTestNullStringRef](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestNullStringRef{{String: "abc", Valid: true}, {}}, named)

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	nullStrings, err := ss.db.From("items").Select("data").ScanAllVals[*sql.NullString](context.Background())
	ss.NoError(err)
	ss.Equal([]*sql.NullString{{String: "abc", Valid: true}, {}}, nullStrings)

	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)).AddRow(nil))
	ids, err := ss.db.From("items").Select("id").ScanAllVals[**int64](context.Background())
	ss.NoError(err)
	ss.Require().Len(ids, 2)
	ss.Equal(int64(3), **ids[0])
	ss.Nil(*ids[1])
}

func (ss *scanSuite) TestScanAllValsRejectsNonNullablePointers() {
	ids, err := ss.db.From("items").ScanAllVals[*int64](context.Background())
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into *int64, whose element int64 cannot hold NULL; use sql.Null[int64]")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.Nil(ids)

	for name, err := range map[string]error{
		"string":                 allValsErr[*string](ss),
		"bool":                   allValsErr[*bool](ss),
		"float64":                allValsErr[*float64](ss),
		"time.Time":              allValsErr[*time.Time](ss),
		"goqu_test.ScanTestTime": allValsErr[*ScanTestTime](ss),
		"goqu_test.scanTestBlob": allValsErr[*scanTestBlob](ss),
	} {
		ss.EqualError(err, "goqu: ScanAllVals cannot scan into *"+name+", whose element "+name+" cannot hold NULL; use sql.Null["+name+"]", name)
	}

	for msg, err := range map[string]error{
		"*fmt.Stringer; use a sql.Scanner": allValsErr[*fmt.Stringer](ss),
		"*uintptr; use a sql.Scanner":      allValsErr[*uintptr](ss),
		"*complex128; use a sql.Scanner":   allValsErr[*complex128](ss),
		"*goqu_test.ScanTestItem, which is a row struct; use ScanAllStructs[*goqu_test.ScanTestItem]": allValsErr[*ScanTestItem](ss),
		"*[]int64; use []byte or a sql.Scanner":                                                       allValsErr[*[]int64](ss),
		"**[]int64; use []byte or a sql.Scanner":                                                      allValsErr[**[]int64](ss),
	} {
		ss.EqualError(err, "goqu: ScanAllVals cannot scan into "+msg, msg)
		ss.True(exec.IsUnsupportedScanType(err), msg)
	}
}

func (ss *scanSuite) TestScanAllStructsAcceptsStructFieldsAndNonStructEmbeds() {
	created := time.Unix(1700000000, 0).UTC()
	ss.mock.ExpectQuery(`SELECT "created", "note" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"created", "note"}).AddRow(created, nil))
	ss.mock.ExpectQuery(`SELECT "scantestname" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"scantestname"}).AddRow("a"))

	times, err := ss.db.From("items").ScanAllStructs[scanTestTimeRow](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestTimeRow{{Created: created}}, times)

	named, err := ss.db.From("items").ScanAllStructs[scanTestNamed](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestNamed{{ScanTestName: "a"}}, named)
}

func (ss *scanSuite) TestScanAllStructsAcceptsDatabaseSQLRowStructs() {
	ss.mock.ExpectQuery(`SELECT "isolation", "readonly" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"isolation", "readonly"}).AddRow(int64(2), true))

	opts, err := ss.db.From("items").ScanAllStructs[sql.TxOptions](context.Background())
	ss.NoError(err)
	ss.Equal([]sql.TxOptions{{Isolation: sql.LevelReadCommitted, ReadOnly: true}}, opts)
}

func (ss *scanSuite) TestScanAllStructsAcceptsSkippedEmbeddedNonStructPointers() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)))
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(6)))

	skipped, err := ss.db.From("items").ScanAllStructs[scanTestSkippedID](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestSkippedID{{ID: 5}}, skipped)

	nested, err := ss.db.From("items").ScanAllStructs[scanTestSkippedNested](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestSkippedNested{{ID: 6}}, nested)
}

func (ss *scanSuite) TestScanStructsRejectUnscannableFields() {
	_, _, err := ss.db.From("items").ScanOneStruct[scanTestNestedIDPtr](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.scanTestNestedIDPtr: embedded field scanTestEmbeddedID.ScanTestID "+
		"is a pointer to non-struct goqu_test.ScanTestID; tag it `db:\"-\"`")

	_, err = ss.db.From("items").ScanAllStructs[*ScanTestNode](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into *goqu_test.ScanTestNode: field Parent leads back to goqu_test.ScanTestNode; "+
		"tag it `db:\"-\"`")

	nonStruct := "is a pointer to non-struct goqu_test.ScanTestID; tag it `db:\"-\"`"
	for msg, err := range map[string]error{
		"scanTestEmbeddedID: embedded field ScanTestID " + nonStruct:                  allStructsErr[scanTestEmbeddedID](ss),
		"scanTestNestedID: embedded field scanTestEmbeddedID.ScanTestID " + nonStruct: allStructsErr[scanTestNestedID](ss),
		"scanTestJSONSkip: embedded field ScanTestID " + nonStruct:                    allStructsErr[scanTestJSONSkip](ss),
		"scanTestAuditRow: embedded field Audit.ScanTestID " + nonStruct:              allStructsErr[scanTestAuditRow](ss),
		"scanTestUnexportedPtr: embedded field dsTestActionItem is a pointer to unexported goqu_test.dsTestActionItem; " +
			"embed it by value or tag it `db:\"-\"`": allStructsErr[scanTestUnexportedPtr](ss),
		"ScanTestSelfRef: field ScanTestSelfRef leads back to goqu_test.ScanTestSelfRef; tag it `db:\"-\"`": allStructsErr[ScanTestSelfRef](ss),
		"ScanTestNode: field Parent leads back to goqu_test.ScanTestNode; tag it `db:\"-\"`":                allStructsErr[ScanTestNode](ss),
		"scanTestNodeRow: field Node.Parent leads back to goqu_test.ScanTestNode; tag it `db:\"-\"`":        allStructsErr[scanTestNodeRow](ss),
	} {
		ss.EqualError(err, "goqu: cannot scan rows into goqu_test."+msg, msg)
		ss.True(exec.IsUnsupportedScanType(err), msg)
	}
}

func (ss *scanSuite) TestScanAllStructsAcceptsSkippedSelfReferences() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	trees, err := ss.db.From("items").ScanAllStructs[ScanTestTree](context.Background())
	ss.NoError(err)
	ss.Equal([]ScanTestTree{{ID: 1, children: nil}}, trees)
}

func (ss *scanSuite) TestScanAllStructsAcceptsUnexportedPointerEmbedsWithoutColumns() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))

	rows, err := ss.db.From("items").ScanAllStructs[scanTestHiddenMeta](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestHiddenMeta{{ID: 2}}, rows)
}

func (ss *scanSuite) TestScanOneValAcceptsBytes() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")))
	ss.mock.ExpectQuery(`SELECT "data" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("def")))

	data, found, err := ss.db.From("items").Select("data").ScanOneVal[[]byte](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal([]byte("abc"), data)

	ptr, found, err := ss.db.From("items").Select("data").ScanOneVal[*[]byte](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal([]byte("def"), *ptr)
}

func (ss *scanSuite) TestScanAllValsAcceptsNamedByteSlicesAndScanners() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte(`{"a":1}`)))
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")))

	docs, err := ss.db.From("items").Select("data").ScanAllVals[json.RawMessage](context.Background())
	ss.NoError(err)
	ss.Equal([]json.RawMessage{json.RawMessage(`{"a":1}`)}, docs)

	all, err := ss.db.From("items").Select("data").ScanAllVals[sql.NullString](context.Background())
	ss.NoError(err)
	ss.Equal([]sql.NullString{{String: "abc", Valid: true}}, all)
}

func (ss *scanSuite) TestScanOneValAcceptsSliceScanners() {
	ss.mock.ExpectQuery(`SELECT "tags" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"tags"}).AddRow([]byte("a,b")))

	tags, found, err := ss.db.From("items").Select("tags").ScanOneVal[scanTestTags](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTags{"a", "b"}, tags)

	ss.mock.ExpectQuery(`SELECT "tags" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"tags"}).AddRow([]byte("c")))
	tagsPtr, found, err := ss.db.From("items").Select("tags").ScanOneVal[*scanTestTags](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTags{"c"}, *tagsPtr)
}

func (ss *scanSuite) TestScanOneValAcceptsByteSliceScanners() {
	ss.mock.ExpectQuery(`SELECT "doc" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow([]byte("abc")))

	doc, found, err := ss.db.From("items").Select("doc").ScanOneVal[scanTestScannedBlob](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestScannedBlob("abc"), doc)
}

func (ss *scanSuite) TestScanAllStructsAcceptsRepeatedStructFields() {
	ss.mock.ExpectQuery(`SELECT "from"."x" AS "from.x", "to"."x" AS "to.x" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"from.x", "to.x"}).AddRow(int64(1), int64(2)))

	segments, err := ss.db.From("items").ScanAllStructs[scanTestSegment](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestSegment{{From: ScanTestPoint{X: 1}, To: ScanTestPoint{X: 2}}}, segments)
}

func (ss *scanSuite) TestScanOneStructKeepsExplicitSelect() {
	ss.mock.ExpectQuery(`SELECT "name" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Test1"))

	item, found, err := ss.db.From("items").Select("name").ScanOneStruct[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneValAcceptsPointersToNamedByteSlices() {
	ss.mock.ExpectQuery(`SELECT "doc" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow([]byte(`{"a":1}`)))
	ss.mock.ExpectQuery(`SELECT "doc" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow(nil))

	doc, found, err := ss.db.From("items").Select("doc").ScanOneVal[*json.RawMessage](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(json.RawMessage(`{"a":1}`), *doc)

	doc, found, err = ss.db.From("items").Select("doc").ScanOneVal[*json.RawMessage](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Nil(doc)
}

func (ss *scanSuite) TestScanOneValWithoutDatabase() {
	_, _, err := goqu.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(scanSuite))
}
