package goqu_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/netip"
	"strings"
	"testing"
	"time"
	"unsafe"
	"uuid"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
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
	ScanTestUUID           uuid.UUID
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
	scanTestByte        uint8
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
	scanTestIgnoredEmbed struct {
		scanTestAuditID `db:"-"`
	}
	scanTestPrivateID struct {
		id int64 `db:"id"`
	}
	scanTestHiddenMeta struct {
		*scanTestMeta
		ID int64 `db:"id"`
	}
	scanTestAuditID struct {
		ID int64 `db:"id"`
	}
	scanTestShadowedAudit struct {
		ID int64 `db:"id"`
		*scanTestAuditID
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

func (ss *scanSuite) allStructsErr[T any]() error {
	rows, err := ss.db.From("items").ScanAllStructs[T](context.Background())
	ss.Nil(rows)
	return err
}

func (ss *scanSuite) allValsErr[T any]() error {
	vals, err := ss.db.From("items").ScanAllVals[T](context.Background())
	ss.Nil(vals)
	return err
}

func (ss *scanSuite) oneValErr[T any]() error {
	_, found, err := ss.db.From("items").ScanOneVal[T](context.Background())
	ss.False(found)
	return err
}

func (ss *scanSuite) iterStructsErr[T any]() error {
	rows, errs := ss.collect(ss.db.From("items").IterStructs[T](context.Background()))
	ss.Empty(rows)
	ss.Require().Len(errs, 1)
	return errs[0]
}

func (ss *scanSuite) iterValsErr[T any]() error {
	vals, errs := ss.collect(ss.db.From("items").Select("id").IterVals[T](context.Background()))
	ss.Empty(vals)
	ss.Require().Len(errs, 1)
	return errs[0]
}

func (ss *scanSuite) collect[T any](seq iter.Seq2[T, error]) (rows []T, errs []error) {
	for row, err := range seq {
		if err != nil {
			ss.Zero(row)
			errs = append(errs, err)
			continue
		}
		rows = append(rows, row)
	}
	return rows, errs
}

type Null[T any] struct{ V T }

func (n *Null[T]) Scan(src interface{}) error {
	return json.Unmarshal([]byte(`{"`+src.(string)+`":1}`), &n.V)
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

func (ss *scanSuite) TestScanAllStructs_pointers() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	items, err := ss.db.From("items").ScanAllStructs[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]*dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructs_prepared() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" WHERE ("name" = ?)`).
		WithArgs("Test1").
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.From("items").Prepared(true).Where(goqu.C("name").Eq("Test1"))
	items, err := ds.ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructs_noRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows(""))

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructs_queryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(scanTestError{})

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructs_rowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2").RowError(1, scanTestError{}))

	items, err := ss.db.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllStructs_fromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("address", "name") VALUES ('111 Test Addr', 'Test1') RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, err := ds.Executor().ScanAllStructs[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllStructs_requiresStruct() {
	items, err := ss.db.From("items").ScanAllStructs[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string, a struct or a pointer to a struct is required")
	ss.True(goqu.IsUnsupportedScanType(err))
	ss.Nil(items)
}

func (ss *scanSuite) TestExecutor_rejectsUnscannableTypes() {
	executor := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}).Returning("name").Executor()

	names, err := executor.ScanAllStructs[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string, a struct or a pointer to a struct is required")
	ss.True(goqu.IsUnsupportedScanType(err))
	ss.Nil(names)

	_, found, err := executor.ScanOneStruct[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string, a struct or a pointer to a struct is required")
	ss.False(found)

	ids, err := executor.ScanAllVals[[]int64](context.Background())
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into []int64; use []byte or a concrete sql.Scanner")
	ss.Nil(ids)

	_, found, err = executor.ScanOneVal[[]int64](context.Background())
	ss.EqualError(err, "goqu: ScanOneVal cannot scan into []int64; use []byte or a concrete sql.Scanner")
	ss.False(found)

	names, errs := ss.collect(executor.IterStructs[string](context.Background()))
	ss.Empty(names)
	ss.Require().Len(errs, 1)
	ss.EqualError(errs[0], "goqu: cannot scan rows into string, a struct or a pointer to a struct is required")

	idLists, errs := ss.collect(executor.IterVals[[]int64](context.Background()))
	ss.Empty(idLists)
	ss.Require().Len(errs, 1)
	ss.EqualError(errs[0], "goqu: IterVals cannot scan into []int64; use []byte or a concrete sql.Scanner")
}

func (ss *scanSuite) TestScanAllStructs_withoutDatabase() {
	_, err := goqu.From("items").ScanAllStructs[dsTestActionItem](context.Background())
	ss.True(goqu.IsNoDatabase(err))
}

func (ss *scanSuite) TestScanOneStruct() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStruct_pointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStruct_pointerNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := ss.db.From("items").ScanOneStruct[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneStruct_notFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneStruct_queryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneStruct_rowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1").RowError(0, scanTestError{}))

	item, found, err := ss.db.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneStruct_fromExecutor() {
	ss.mock.ExpectQuery(`UPDATE "items" SET "name"='Test2' RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test2"))

	ds := ss.db.Update("items").Set(goqu.Record{"name": "Test2"}).Returning("address", "name")
	item, found, err := ds.Executor().ScanOneStruct[*dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test2"}, item)
}

func (ss *scanSuite) TestScanOneStruct_requiresStruct() {
	_, found, err := ss.db.From("items").ScanOneStruct[string](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into string, a struct or a pointer to a struct is required")
	ss.True(goqu.IsUnsupportedScanType(err))
	ss.False(found)

	row, found, err := ss.db.From("items").ScanOneStruct[**dsTestActionItem](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into **goqu_test.dsTestActionItem, a struct or a pointer to a struct is required")
	ss.False(found)
	ss.Nil(row)
}

func (ss *scanSuite) TestStructScans_rejectsSingleColumnTypes() {
	_, err := ss.db.From("items").ScanAllStructs[time.Time](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into time.Time, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[time.Time]/ScanOneVal[time.Time] or IterVals[time.Time]")
	ss.True(goqu.IsUnsupportedScanType(err))

	_, err = ss.db.From("items").ScanAllStructs[*time.Time](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into *time.Time, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[time.Time]/ScanOneVal[time.Time] or IterVals[time.Time]")

	_, _, err = ss.db.From("items").ScanOneStruct[sql.NullString](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into sql.NullString, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[sql.NullString]/ScanOneVal[sql.NullString] or IterVals[sql.NullString]")

	_, _, err = ss.db.From("items").ScanOneStruct[*sql.NullString](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into *sql.NullString, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[sql.NullString]/ScanOneVal[sql.NullString] or IterVals[sql.NullString]")

	_, err = ss.db.From("items").ScanAllStructs[scanTestOpaqueScanner](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.scanTestOpaqueScanner, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[goqu_test.scanTestOpaqueScanner]/ScanOneVal[goqu_test.scanTestOpaqueScanner] "+
		"or IterVals[goqu_test.scanTestOpaqueScanner]")

	_, err = ss.db.From("items").ScanAllStructs[ScanTestTime](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.ScanTestTime, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[goqu_test.ScanTestTime]/ScanOneVal[goqu_test.ScanTestTime] or IterVals[goqu_test.ScanTestTime]")
}

func (ss *scanSuite) TestScanAllStructs_rejectsStructsWithoutColumns() {
	for name, err := range map[string]error{
		"netip.Addr":                    ss.allStructsErr[netip.Addr](),
		"goqu_test.scanTestEmbedded":    ss.allStructsErr[scanTestEmbedded](),
		"goqu_test.scanTestEmbeddedPtr": ss.allStructsErr[scanTestEmbeddedPtr](),
		"strings.Builder":               ss.allStructsErr[strings.Builder](),
	} {
		ss.EqualError(err, "goqu: cannot scan rows into "+name+", which has no fields goqu can scan into", name)
		ss.True(goqu.IsUnsupportedScanType(err), name)
	}

	ss.EqualError(ss.allStructsErr[*strings.Builder](), "goqu: cannot scan rows into *strings.Builder, which has no fields goqu can scan into")
}

func (ss *scanSuite) TestScanAllStructs_acceptsEmbeddedRowFields() {
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

func (ss *scanSuite) TestGenericScans_useDatasetTagName() {
	goqu.SetIgnoreUntaggedFields(true)
	defer goqu.SetIgnoreUntaggedFields(false)
	mDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	ss.Require().NoError(err)
	mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(8)))
	mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	db := goqu.NewWithTagName("mock", "json", mDB)

	rows, err := db.From("items").ScanAllStructs[scanTestTagged](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestTagged{{ID: 7}}, rows)

	row, found, err := db.From("items").ScanOneStruct[scanTestTagged](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTagged{ID: 8}, row)

	iterated, errs := ss.collect(db.From("items").IterStructs[scanTestTagged](context.Background()))
	ss.Empty(errs)
	ss.Equal([]scanTestTagged{{ID: 9}}, iterated)
	ss.NoError(mock.ExpectationsWereMet())

	_, err = db.From("items").ScanAllVals[*scanTestTagged](context.Background())
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into *goqu_test.scanTestTagged: "+
		"goqu_test.scanTestTagged is a row struct; use ScanAllStructs[*goqu_test.scanTestTagged]")

	_, iterErrs := ss.collect(db.From("items").IterVals[scanTestTagged](context.Background()))
	ss.Require().Len(iterErrs, 1)
	_, _, oneErr := db.From("items").ScanOneVal[scanTestTagged](context.Background())
	_, allErr := db.From("items").ScanAllVals[scanTestTagged](context.Background())
	const tagged = "goqu_test.scanTestTagged"
	for want, err := range map[string]error{
		"IterVals cannot scan into " + tagged + ": " + tagged + " is a row struct; use IterStructs[" + tagged + "]":       iterErrs[0],
		"ScanOneVal cannot scan into " + tagged + ": " + tagged + " is a row struct; use ScanOneStruct[" + tagged + "]":   oneErr,
		"ScanAllVals cannot scan into " + tagged + ": " + tagged + " is a row struct; use ScanAllStructs[" + tagged + "]": allErr,
	} {
		ss.EqualError(err, "goqu: "+want)
	}

	_, err = db.From("items").ScanAllStructs[scanTestDBSkip](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.scanTestDBSkip: embedded field ScanTestID is a pointer to "+
		"non-struct goqu_test.ScanTestID; tag it `json:\"-\"`")
}

func (ss *scanSuite) TestScanOneStruct_acceptsRowStructWithScan() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[scanTestScannerRow](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestScannerRow{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneStruct_namedPointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := ss.db.From("items").ScanOneStruct[scanTestItemRef](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestItemRef(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}), item)
}

func (ss *scanSuite) TestScanOneStruct_pointerQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	item, found, err := ss.db.From("items").ScanOneStruct[*dsTestActionItem](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneStruct_withoutDatabase() {
	_, _, err := goqu.From("items").ScanOneStruct[dsTestActionItem](context.Background())
	ss.True(goqu.IsNoDatabase(err))
}

func (ss *scanSuite) TestScanAllVals() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanAllVals_noRows() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.NoError(err)
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllVals_queryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnError(scanTestError{})

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllVals_rowError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2").RowError(1, scanTestError{}))

	ids, err := ss.db.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanAllVals_fromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("name") VALUES ('Test1'), ('Test2') RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ds := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}, goqu.Record{"name": "Test2"}).Returning("id")
	ids, err := ds.Executor().ScanAllVals[int64](context.Background())
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanAllVals_withoutDatabase() {
	_, err := goqu.From("items").Select("id").ScanAllVals[int64](context.Background())
	ss.True(goqu.IsNoDatabase(err))
}

func (ss *scanSuite) TestScanOneVal() {
	ss.mock.ExpectQuery(`SELECT COUNT(*) AS "count" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("10"))

	count, found, err := ss.db.From("items").Select(goqu.COUNT("*").As("count")).ScanOneVal[int64](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(10), count)
}

func (ss *scanSuite) TestScanOneVal_notFound() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	id, found, err := ss.db.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.NoError(err)
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanOneVal_queryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := ss.db.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneVal_rowError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1").RowError(0, scanTestError{}))

	id, found, err := ss.db.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanOneVal_fromExecutor() {
	ss.mock.ExpectQuery(`DELETE FROM "items" RETURNING "id"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("7"))

	id, found, err := ss.db.Delete("items").Returning("id").Executor().ScanOneVal[int64](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(7), id)
}

func (ss *scanSuite) TestScanOneVal_rejectsSlices() {
	for name, err := range map[string]error{
		"[]int64":                   ss.oneValErr[[]int64](),
		"*[]int64":                  ss.oneValErr[*[]int64](),
		"**[]int64":                 ss.oneValErr[**[]int64](),
		"[]goqu_test.scanTestByte":  ss.oneValErr[[]scanTestByte](),
		"*[]goqu_test.scanTestByte": ss.oneValErr[*[]scanTestByte](),
	} {
		ss.EqualError(err, "goqu: ScanOneVal cannot scan into "+name+"; use []byte or a concrete sql.Scanner", name)
		ss.True(goqu.IsUnsupportedScanType(err), name)
	}
}

func (ss *scanSuite) TestScanOneVal_acceptsNamedByteSliceValues() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte(`{"a":1}`)))
	doc, found, err := ss.db.From("items").Select("data").ScanOneVal[json.RawMessage](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(json.RawMessage(`{"a":1}`), doc)
}

func (ss *scanSuite) TestScanOneVal_rejectsRawBytes() {
	for _, err := range []error{ss.oneValErr[sql.RawBytes](), ss.oneValErr[*sql.RawBytes](), ss.oneValErr[**sql.RawBytes]()} {
		ss.EqualError(err, "goqu: ScanOneVal cannot return sql.RawBytes, "+
			"which is only valid until the next row is read or the rows are closed; use []byte")
		ss.True(goqu.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestScanAllVals_rejectsRawBytes() {
	for _, err := range []error{ss.allValsErr[sql.RawBytes](), ss.allValsErr[*sql.RawBytes](), ss.allValsErr[**sql.RawBytes]()} {
		ss.EqualError(err, "goqu: ScanAllVals cannot return sql.RawBytes, "+
			"which is only valid until the next row is read or the rows are closed; use []byte")
		ss.True(goqu.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestScanAllVals_pointersReceiveNilForNull() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)).AddRow(nil))
	ids, err := ss.db.From("items").Select("id").ScanAllVals[*int64](context.Background())
	ss.NoError(err)
	ss.Require().Len(ids, 2)
	ss.Equal(int64(3), *ids[0])
	ss.Nil(ids[1])

	ss.mock.ExpectQuery(`SELECT "name" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("a").AddRow(nil))
	names, err := ss.db.From("items").Select("name").ScanAllVals[*string](context.Background())
	ss.NoError(err)
	ss.Require().Len(names, 2)
	ss.Equal("a", *names[0])
	ss.Nil(names[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte("abc")).AddRow(nil))
	data, err := ss.db.From("items").Select("data").ScanAllVals[*[]byte](context.Background())
	ss.NoError(err)
	ss.Require().Len(data, 2)
	ss.Equal([]byte("abc"), *data[0])
	ss.Nil(data[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	anys, err := ss.db.From("items").Select("data").ScanAllVals[*any](context.Background())
	ss.NoError(err)
	ss.Require().Len(anys, 2)
	ss.Equal("abc", *anys[0])
	ss.Nil(anys[1])

	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	nullStrings, err := ss.db.From("items").Select("data").ScanAllVals[*sql.NullString](context.Background())
	ss.NoError(err)
	ss.Equal([]*sql.NullString{{String: "abc", Valid: true}, nil}, nullStrings)

	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)).AddRow(nil))
	idRefs, err := ss.db.From("items").Select("id").ScanAllVals[**int64](context.Background())
	ss.NoError(err)
	ss.Require().Len(idRefs, 2)
	ss.Equal(int64(3), **idRefs[0])
	ss.Nil(idRefs[1])
}

func (ss *scanSuite) TestScanAllVals_leavesStructsWithoutColumnTagsToTheDriver() {
	ss.Equal(int64(1), scanTestPrivateID{id: 1}.id)
	for name, scan := range map[string]func() error{
		"ScanTestSelfRef":      ss.allValsErr[ScanTestSelfRef],
		"scanTestMeta":         ss.allValsErr[scanTestMeta],
		"scanTestIgnoredEmbed": ss.allValsErr[scanTestIgnoredEmbed],
		"scanTestPrivateID":    ss.allValsErr[scanTestPrivateID],
	} {
		ss.mock.ExpectQuery(`SELECT * FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow("a"))
		err := scan()
		ss.ErrorContains(err, "sql: Scan error on column index 0", name)
		ss.False(goqu.IsUnsupportedScanType(err), name)
	}
}

func (ss *scanSuite) TestScanAllVals_namedPointersReceiveNilForNull() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow("abc").AddRow(nil))
	named, err := ss.db.From("items").Select("data").ScanAllVals[scanTestNullStringRef](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestNullStringRef{{String: "abc", Valid: true}, nil}, named)
}

func (ss *scanSuite) TestScanAllVals_rejectsUnscannablePointers() {
	for msg, err := range map[string]error{
		"*uintptr; use a concrete sql.Scanner":                            ss.allValsErr[*uintptr](),
		"*complex128; use a concrete sql.Scanner":                         ss.allValsErr[*complex128](),
		"*[]int64; use []byte or a concrete sql.Scanner":                  ss.allValsErr[*[]int64](),
		"**[]int64; use []byte or a concrete sql.Scanner":                 ss.allValsErr[**[]int64](),
		"*[]goqu_test.scanTestByte; use []byte or a concrete sql.Scanner": ss.allValsErr[*[]scanTestByte](),
	} {
		ss.EqualError(err, "goqu: ScanAllVals cannot scan into "+msg, msg)
		ss.True(goqu.IsUnsupportedScanType(err), msg)
	}
}

func (ss *scanSuite) TestScanAllVals_rejectsRowStructPointers() {
	for elem, err := range map[string]error{
		"goqu_test.ScanTestItem":          ss.allValsErr[*ScanTestItem](),
		"goqu_test.ScanTestNode":          ss.allValsErr[*ScanTestNode](),
		"goqu_test.scanTestEmbeddedID":    ss.allValsErr[*scanTestEmbeddedID](),
		"goqu_test.scanTestUnexportedPtr": ss.allValsErr[*scanTestUnexportedPtr](),
	} {
		ss.EqualError(err, "goqu: ScanAllVals cannot scan into *"+elem+": "+elem+
			" is a row struct; use ScanAllStructs[*"+elem+"]", elem)
	}
}

func (ss *scanSuite) TestScanAllStructs_acceptsStructFieldsAndNonStructEmbeds() {
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

func (ss *scanSuite) TestScanAllStructs_acceptsDatabaseSQLRowStructs() {
	ss.mock.ExpectQuery(`SELECT "isolation", "readonly" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"isolation", "readonly"}).AddRow(int64(2), true))

	opts, err := ss.db.From("items").ScanAllStructs[sql.TxOptions](context.Background())
	ss.NoError(err)
	ss.Equal([]sql.TxOptions{{Isolation: sql.LevelReadCommitted, ReadOnly: true}}, opts)
}

func (ss *scanSuite) TestScanAllStructs_acceptsSkippedEmbeddedNonStructPointers() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)))
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(6)))

	skipped, err := ss.db.From("items").ScanAllStructs[scanTestSkippedID](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestSkippedID{{ID: 5}}, skipped)

	nested, err := ss.db.From("items").ScanAllStructs[scanTestSkippedNested](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestSkippedNested{{ID: 6}}, nested)
}

func (ss *scanSuite) TestStructScans_rejectsUnscannableFields() {
	_, _, err := ss.db.From("items").ScanOneStruct[scanTestNestedIDPtr](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into goqu_test.scanTestNestedIDPtr: embedded field scanTestEmbeddedID.ScanTestID "+
		"is a pointer to non-struct goqu_test.ScanTestID; tag it `db:\"-\"`")

	_, err = ss.db.From("items").ScanAllStructs[*ScanTestNode](context.Background())
	ss.EqualError(err, "goqu: cannot scan rows into *goqu_test.ScanTestNode: field Parent leads back to goqu_test.ScanTestNode; "+
		"tag it `db:\"-\"`")

	ss.EqualError(ss.allStructsErr[*scanTestEmbeddedID](), "goqu: cannot scan rows into *goqu_test.scanTestEmbeddedID: "+
		"embedded field ScanTestID is a pointer to non-struct goqu_test.ScanTestID; tag it `db:\"-\"`")
	ss.EqualError(ss.allStructsErr[*scanTestUnexportedPtr](), "goqu: cannot scan rows into *goqu_test.scanTestUnexportedPtr: "+
		"embedded field dsTestActionItem is a pointer to unexported goqu_test.dsTestActionItem; embed it by value or tag it `db:\"-\"`")

	nonStruct := "is a pointer to non-struct goqu_test.ScanTestID; tag it `db:\"-\"`"
	for msg, err := range map[string]error{
		"scanTestEmbeddedID: embedded field ScanTestID " + nonStruct:                  ss.allStructsErr[scanTestEmbeddedID](),
		"scanTestNestedID: embedded field scanTestEmbeddedID.ScanTestID " + nonStruct: ss.allStructsErr[scanTestNestedID](),
		"scanTestJSONSkip: embedded field ScanTestID " + nonStruct:                    ss.allStructsErr[scanTestJSONSkip](),
		"scanTestAuditRow: embedded field Audit.ScanTestID " + nonStruct:              ss.allStructsErr[scanTestAuditRow](),
		"scanTestUnexportedPtr: embedded field dsTestActionItem is a pointer to unexported goqu_test.dsTestActionItem; " +
			"embed it by value or tag it `db:\"-\"`": ss.allStructsErr[scanTestUnexportedPtr](),
		"ScanTestSelfRef: field ScanTestSelfRef leads back to goqu_test.ScanTestSelfRef; tag it `db:\"-\"`": ss.allStructsErr[ScanTestSelfRef](),
		"ScanTestNode: field Parent leads back to goqu_test.ScanTestNode; tag it `db:\"-\"`":                ss.allStructsErr[ScanTestNode](),
		"scanTestNodeRow: field Node.Parent leads back to goqu_test.ScanTestNode; tag it `db:\"-\"`":        ss.allStructsErr[scanTestNodeRow](),
	} {
		ss.EqualError(err, "goqu: cannot scan rows into goqu_test."+msg, msg)
		ss.True(goqu.IsUnsupportedScanType(err), msg)
	}
}

func (ss *scanSuite) TestScanAllStructs_acceptsSkippedSelfReferences() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	trees, err := ss.db.From("items").ScanAllStructs[ScanTestTree](context.Background())
	ss.NoError(err)
	ss.Equal([]ScanTestTree{{ID: 1, children: nil}}, trees)
}

func (ss *scanSuite) TestScanAllStructs_acceptsUnexportedPointerEmbedsWithoutColumns() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3)))

	rows, err := ss.db.From("items").ScanAllStructs[scanTestHiddenMeta](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestHiddenMeta{{ID: 2}}, rows)

	shadowed, err := ss.db.From("items").ScanAllStructs[scanTestShadowedAudit](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestShadowedAudit{{ID: 3}}, shadowed)
}

func (ss *scanSuite) TestScanOneVal_acceptsBytes() {
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

func (ss *scanSuite) TestScanAllVals_acceptsNamedByteSlicesAndScanners() {
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

func (ss *scanSuite) TestScanOneVal_acceptsSliceScanners() {
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

func (ss *scanSuite) TestScanOneVal_acceptsByteSliceScanners() {
	ss.mock.ExpectQuery(`SELECT "doc" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow([]byte("abc")))

	doc, found, err := ss.db.From("items").Select("doc").ScanOneVal[scanTestScannedBlob](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestScannedBlob("abc"), doc)
}

func (ss *scanSuite) TestScanAllStructs_acceptsRepeatedStructFields() {
	ss.mock.ExpectQuery(`SELECT "from"."x" AS "from.x", "to"."x" AS "to.x" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"from.x", "to.x"}).AddRow(int64(1), int64(2)))

	segments, err := ss.db.From("items").ScanAllStructs[scanTestSegment](context.Background())
	ss.NoError(err)
	ss.Equal([]scanTestSegment{{From: ScanTestPoint{X: 1}, To: ScanTestPoint{X: 2}}}, segments)
}

func (ss *scanSuite) TestScanOneStruct_keepsExplicitSelect() {
	ss.mock.ExpectQuery(`SELECT "name" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Test1"))

	item, found, err := ss.db.From("items").Select("name").ScanOneStruct[dsTestActionItem](context.Background())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneVal_acceptsPointersToNamedByteSlices() {
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

func (ss *scanSuite) TestScanOneVal_withoutDatabase() {
	_, _, err := goqu.From("items").Select("id").ScanOneVal[int64](context.Background())
	ss.True(goqu.IsNoDatabase(err))
}

func (ss *scanSuite) TestIterStructs() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2"))

	items, errs := ss.collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestIterStructs_pointers() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2"))

	items, errs := ss.collect(ss.db.From("items").IterStructs[*dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Equal([]*dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestIterStructs_namedPointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	items, errs := ss.collect(ss.db.From("items").IterStructs[scanTestItemRef](context.Background()))
	ss.Empty(errs)
	ss.Equal([]scanTestItemRef{&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestIterStructs_breakClosesRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2")).
		RowsWillBeClosed()

	var items []dsTestActionItem
	for item, err := range ss.db.From("items").IterStructs[dsTestActionItem](context.Background()) {
		ss.Require().NoError(err)
		items = append(items, item)
		break
	}
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestIterStructs_runsTheQueryOnEveryRange() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("211 Test Addr,Test2"))

	seq := ss.db.From("items").IterStructs[dsTestActionItem](context.Background())
	first, firstErrs := ss.collect(seq)
	second, secondErrs := ss.collect(seq)
	ss.Empty(append(firstErrs, secondErrs...))
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, first)
	ss.Equal([]dsTestActionItem{{Address: "211 Test Addr", Name: "Test2"}}, second)
}

func (ss *scanSuite) TestIterStructs_noRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows(""))

	items, errs := ss.collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Empty(items)
}

func (ss *scanSuite) TestIterStructs_queryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(scanTestError{})

	items, errs := ss.collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(items)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterStructs_rowErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2").RowError(1, scanTestError{}))

	items, errs := ss.collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterStructs_scanErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "test" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("a\nb")).
		RowsWillBeClosed()

	items, errs := ss.collect(ss.db.From("items").Select("test").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(items)
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsColumnMismatch(errs[0]))
}

func (ss *scanSuite) TestIterStructs_pointerScanErrorYieldsNil() {
	ss.mock.ExpectQuery(`SELECT "address", "test" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "test"}).FromCSVString("111 Test Addr,a"))

	items, errs := ss.collect(ss.db.From("items").Select("address", "test").IterStructs[*dsTestActionItem](context.Background()))
	ss.Empty(items)
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsColumnMismatch(errs[0]))
}

func (ss *scanSuite) TestIterStructs_requiresStruct() {
	err := ss.iterStructsErr[string]()
	ss.EqualError(err, "goqu: cannot scan rows into string, a struct or a pointer to a struct is required")
	ss.True(goqu.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestIterStructs_rejectsSingleColumnStructs() {
	err := ss.iterStructsErr[sql.NullString]()
	ss.EqualError(err, "goqu: cannot scan rows into sql.NullString, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[sql.NullString]/ScanOneVal[sql.NullString] or IterVals[sql.NullString]")
	ss.True(goqu.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestIterStructs_fromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("address", "name") VALUES ('111 Test Addr', 'Test1') RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, errs := ss.collect(ds.Executor().IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestIterStructs_withoutDatabase() {
	_, errs := ss.collect(goqu.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsNoDatabase(errs[0]))
}

func (ss *scanSuite) TestIterVals() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Empty(errs)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestIterVals_nullablePointers() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(nil))

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[*int64](context.Background()))
	ss.Empty(errs)
	ss.Require().Len(ids, 2)
	ss.Equal(int64(1), *ids[0])
	ss.Nil(ids[1])
}

func (ss *scanSuite) TestIterVals_breakClosesRows() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2")).
		RowsWillBeClosed()

	var ids []int64
	for id, err := range ss.db.From("items").Select("id").IterVals[int64](context.Background()) {
		ss.Require().NoError(err)
		ids = append(ids, id)
		break
	}
	ss.Equal([]int64{1}, ids)
}

func (ss *scanSuite) TestIterVals_queryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnError(scanTestError{})

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Empty(ids)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterVals_rowErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2").RowError(1, scanTestError{}))

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Equal([]int64{1}, ids)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterVals_scanErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\nb\n3")).
		RowsWillBeClosed()

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Equal([]int64{1}, ids)
	ss.Require().Len(errs, 1)
	ss.ErrorContains(errs[0], `sql: Scan error on column index 0, name "id"`)
}

func (ss *scanSuite) TestIterVals_rejectsUnscannableTypes() {
	for want, err := range map[string]error{
		"goqu: IterVals cannot return sql.RawBytes, which is only valid until the next row is read " +
			"or the rows are closed; use []byte": ss.iterValsErr[sql.RawBytes](),
		"goqu: IterVals cannot scan into []string; use []byte or a concrete sql.Scanner": ss.iterValsErr[[]string](),
	} {
		ss.EqualError(err, want)
		ss.True(goqu.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestIterVals_acceptsNamedByteSlices() {
	ss.mock.ExpectQuery(`SELECT "data" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"data"}).AddRow([]byte(`{"a":1}`)))

	docs, errs := ss.collect(ss.db.From("items").Select("data").IterVals[json.RawMessage](context.Background()))
	ss.Empty(errs)
	ss.Equal([]json.RawMessage{json.RawMessage(`{"a":1}`)}, docs)
}

func (ss *scanSuite) TestIterVals_fromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("name") VALUES ('Test1'), ('Test2') RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ds := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}, goqu.Record{"name": "Test2"}).Returning("id")
	ids, errs := ss.collect(ds.Executor().IterVals[int64](context.Background()))
	ss.Empty(errs)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestIterVals_withoutDatabase() {
	_, errs := ss.collect(goqu.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsNoDatabase(errs[0]))
}

func (ss *scanSuite) TestValueScans_rejectRowStructs() {
	const row = "goqu_test.dsTestActionItem"
	for _, c := range []struct {
		fn, target, suggestion string
		err                    error
	}{
		{"IterVals", row, "IterStructs[" + row + "]", ss.iterValsErr[dsTestActionItem]()},
		{"IterVals", "*" + row, "IterStructs[*" + row + "]", ss.iterValsErr[*dsTestActionItem]()},
		{"IterVals", "**" + row, "IterStructs[*" + row + "]", ss.iterValsErr[**dsTestActionItem]()},
		{"ScanOneVal", row, "ScanOneStruct[" + row + "]", ss.oneValErr[dsTestActionItem]()},
		{"ScanOneVal", "*" + row, "ScanOneStruct[*" + row + "]", ss.oneValErr[*dsTestActionItem]()},
		{"ScanAllVals", row, "ScanAllStructs[" + row + "]", ss.allValsErr[dsTestActionItem]()},
		{"ScanAllVals", "**" + row, "ScanAllStructs[*" + row + "]", ss.allValsErr[**dsTestActionItem]()},
	} {
		ss.EqualError(c.err, "goqu: "+c.fn+" cannot scan into "+c.target+": "+row+" is a row struct; use "+c.suggestion)
		ss.True(goqu.IsUnsupportedScanType(c.err))
	}
}

func (ss *scanSuite) TestValueScans_rejectTypesDatabaseSQLCannotFill() {
	for want, err := range map[string]error{
		"IterVals cannot scan into unsafe.Pointer":            ss.iterValsErr[unsafe.Pointer](),
		"IterVals cannot scan into uintptr":                   ss.iterValsErr[uintptr](),
		"IterVals cannot scan into sql.Scanner":               ss.iterValsErr[sql.Scanner](),
		"IterVals cannot scan into [4]int":                    ss.iterValsErr[[4]int](),
		"ScanOneVal cannot scan into func()":                  ss.oneValErr[func()](),
		"ScanOneVal cannot scan into complex128":              ss.oneValErr[complex128](),
		"ScanOneVal cannot scan into *sql.Scanner":            ss.oneValErr[*sql.Scanner](),
		"ScanOneVal cannot scan into sql.TxOptions":           ss.oneValErr[sql.TxOptions](),
		"ScanOneVal cannot scan into io.Reader":               ss.oneValErr[io.Reader](),
		"ScanOneVal cannot scan into [16]uint8":               ss.oneValErr[[16]byte](),
		"ScanAllVals cannot scan into chan int":               ss.allValsErr[chan int](),
		"ScanAllVals cannot scan into **func()":               ss.allValsErr[**func()](),
		"ScanAllVals cannot scan into complex64":              ss.allValsErr[complex64](),
		"ScanAllVals cannot scan into goqu_test.ScanTestUUID": ss.allValsErr[ScanTestUUID](),
		"ScanAllVals cannot scan into **error":                ss.allValsErr[**error](),
	} {
		ss.EqualError(err, "goqu: "+want+"; use a concrete sql.Scanner")
		ss.True(goqu.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestValueScans_checkTheValueOfSQLNull() {
	const row = "goqu_test.dsTestActionItem"
	for _, c := range []struct {
		want string
		err  error
	}{
		{
			"IterVals cannot scan into the chan int of sql.Null[chan int]; use a concrete sql.Scanner",
			ss.iterValsErr[sql.Null[chan int]](),
		},
		{
			"IterVals cannot scan into the []int64 of *sql.Null[[]int64]; use []byte or a concrete sql.Scanner",
			ss.iterValsErr[*sql.Null[[]int64]](),
		},
		{
			"ScanOneVal cannot return sql.RawBytes, which is only valid until the next row is read " +
				"or the rows are closed; use []byte",
			ss.oneValErr[sql.Null[sql.RawBytes]](),
		},
		{
			"ScanAllVals cannot scan into *sql.Null[github.com/doug-martin/goqu/v9_test.dsTestActionItem]: " +
				row + " is a row struct; use ScanAllStructs[*" + row + "]",
			ss.allValsErr[*sql.Null[dsTestActionItem]](),
		},
	} {
		ss.EqualError(c.err, "goqu: "+c.want)
		ss.True(goqu.IsUnsupportedScanType(c.err))
	}
}

func (ss *scanSuite) TestIterVals_acceptsSQLNull() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(nil))

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[sql.Null[int64]](context.Background()))
	ss.Empty(errs)
	ss.Equal([]sql.Null[int64]{{V: 1, Valid: true}, {}}, ids)
}

func (ss *scanSuite) TestIterVals_acceptsNullScannersFromOtherPackages() {
	ss.mock.ExpectQuery(`SELECT "tags" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"tags"}).AddRow("a"))

	tags, errs := ss.collect(ss.db.From("items").Select("tags").IterVals[Null[map[string]int]](context.Background()))
	ss.Empty(errs)
	ss.Equal([]Null[map[string]int]{{V: map[string]int{"a": 1}}}, tags)
}

func (ss *scanSuite) TestValueScans_rejectCursors() {
	err := ss.iterValsErr[*sql.Rows]()
	ss.EqualError(err, "goqu: IterVals cannot scan into *sql.Rows; use a concrete sql.Scanner")
	ss.True(goqu.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestIterVals_acceptsInterfacesADriverValueImplements() {
	created := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	ss.mock.ExpectQuery(`SELECT "created" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(created))

	vals, errs := ss.collect(ss.db.From("items").Select("created").IterVals[fmt.Stringer](context.Background()))
	ss.Empty(errs)
	ss.Equal([]fmt.Stringer{created}, vals)
}

func (ss *scanSuite) TestIterVals_acceptsUUIDs() {
	id := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id.String()))

	ids, errs := ss.collect(ss.db.From("items").Select("id").IterVals[uuid.UUID](context.Background()))
	ss.Empty(errs)
	ss.Equal([]uuid.UUID{id}, ids)
}

func (ss *scanSuite) TestScanAllVals_rejectsSliceElements() {
	err := ss.allValsErr[[]int64]()
	ss.EqualError(err, "goqu: ScanAllVals cannot scan into []int64; use []byte or a concrete sql.Scanner")
	ss.True(goqu.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestIterVals_acceptsScannerAndTimeStructs() {
	created := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	ss.mock.ExpectQuery(`SELECT "name" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Test1"))
	ss.mock.ExpectQuery(`SELECT "created" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(created))

	names, errs := ss.collect(ss.db.From("items").Select("name").IterVals[sql.NullString](context.Background()))
	ss.Empty(errs)
	ss.Equal([]sql.NullString{{String: "Test1", Valid: true}}, names)

	times, errs := ss.collect(ss.db.From("items").Select("created").IterVals[time.Time](context.Background()))
	ss.Empty(errs)
	ss.Equal([]time.Time{created}, times)
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(scanSuite))
}
