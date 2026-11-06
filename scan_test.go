package goqu_test

import (
	"context"
	"database/sql"
	"encoding/json"
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
	scanTestError    struct{}
	scanTestItemRef  *dsTestActionItem
	scanTestTags     []string
	scanTestJSONItem dsTestActionItem
)

func (t *scanTestTags) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		*t = nil
	case []byte:
		*t = strings.Split(string(v), ",")
	default:
		return scanTestError{}
	}
	return nil
}

func (i *scanTestJSONItem) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return scanTestError{}
	}
	return json.Unmarshal(b, (*dsTestActionItem)(i))
}

func (scanTestError) Error() string { return "scan failed" }

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
	for _, ds := range []*goqu.SelectDataset{ss.db.From("items"), ss.db.From("items").Select("name")} {
		items, err := goqu.ScanAllStructs[string](context.Background(), ds)
		ss.EqualError(err, "goqu: ScanAllStructs requires a struct or a pointer to a struct, got string")
		ss.True(exec.IsUnsupportedScanType(err))
		ss.Nil(items)
	}
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
	for _, ds := range []*goqu.SelectDataset{ss.db.From("items"), ss.db.From("items").Select("name")} {
		_, found, err := goqu.ScanOneStruct[string](context.Background(), ds)
		ss.EqualError(err, "goqu: ScanOneStruct requires a struct or a pointer to a struct, got string")
		ss.True(exec.IsUnsupportedScanType(err))
		ss.False(found)
	}

	row, found, err := goqu.ScanOneStruct[**dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct requires a struct or a pointer to a struct, got **goqu_test.dsTestActionItem")
	ss.False(found)
	ss.Nil(row)
}

func (ss *scanSuite) TestScanRejectsSingleColumnStructs() {
	cases := map[string]func() error{
		"time.Time": func() error {
			_, err := goqu.ScanAllStructs[time.Time](context.Background(), ss.db.From("items"))
			return err
		},
		"*time.Time": func() error {
			_, err := goqu.ScanAllStructs[*time.Time](context.Background(), ss.db.From("items"))
			return err
		},
		"netip.Addr": func() error {
			_, err := goqu.ScanAllStructs[netip.Addr](context.Background(), ss.db.From("items"))
			return err
		},
		"sql.NullString": func() error {
			_, _, err := goqu.ScanOneStruct[sql.NullString](context.Background(), ss.db.From("items"))
			return err
		},
		"*sql.NullString": func() error {
			_, _, err := goqu.ScanOneStruct[*sql.NullString](context.Background(), ss.db.From("items"))
			return err
		},
	}
	for name, scan := range cases {
		err := scan()
		ss.ErrorContains(err, "got "+name+", which scans as one column", name)
		ss.True(exec.IsUnsupportedScanType(err), name)
	}

	_, err := goqu.ScanAllStructs[time.Time](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllStructs requires a row struct, got time.Time, which scans as one column; use ScanAllVals")
	_, _, err = goqu.ScanOneStruct[sql.NullString](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneStruct requires a row struct, got sql.NullString, which scans as one column; use ScanOneVal")
}

func (ss *scanSuite) TestScanStructsAcceptRowStructsWithScan() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOneStruct[scanTestJSONItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestJSONItem{Address: "111 Test Addr", Name: "Test1"}, item)
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
	for _, scan := range []func() error{
		func() error {
			_, _, err := goqu.ScanOneVal[[]int64](context.Background(), ss.db.From("items"))
			return err
		},
		func() error {
			_, _, err := goqu.ScanOneVal[*[]int64](context.Background(), ss.db.From("items"))
			return err
		},
	} {
		err := scan()
		ss.ErrorContains(err, "goqu: ScanOneVal requires a single-column value type, got ")
		ss.True(exec.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestScanValsRejectRawBytes() {
	_, _, err := goqu.ScanOneVal[sql.RawBytes](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOneVal cannot return sql.RawBytes, which is only valid until the next row; use []byte")
	ss.True(exec.IsUnsupportedScanType(err))

	_, err = goqu.ScanAllVals[sql.RawBytes](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot return sql.RawBytes, which is only valid until the next row; use []byte")
	ss.True(exec.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestScanAllValsRejectsPointers() {
	ids, err := goqu.ScanAllVals[*int64](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanAllVals cannot scan NULL into *int64; use sql.Null[int64]")
	ss.True(exec.IsUnsupportedScanType(err))
	ss.Nil(ids)
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

func (ss *scanSuite) TestScanValsAcceptSliceScanners() {
	ss.mock.ExpectQuery(`SELECT "tags" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"tags"}).AddRow([]byte("a,b")))

	tags, found, err := goqu.ScanOneVal[scanTestTags](context.Background(), ss.db.From("items").Select("tags"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(scanTestTags{"a", "b"}, tags)
}

func (ss *scanSuite) TestScanOneValWithoutDatabase() {
	_, _, err := goqu.ScanOneVal[int64](context.Background(), goqu.From("items").Select("id"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(scanSuite))
}
