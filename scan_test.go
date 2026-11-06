package goqu_test

import (
	"context"
	"testing"

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
	scanTestError struct{}
)

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

func (ss *scanSuite) TestScanAll() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2"))

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestScanAllPointers() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	items, err := goqu.ScanAll[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]*dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllPrepared() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" WHERE \("name" = \?\)`).
		WithArgs("Test1").
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.From("items").Prepared(true).Where(goqu.C("name").Eq("Test1"))
	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ds)
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllNoRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows(""))

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(scanTestError{})

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllRowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2").RowError(1, scanTestError{}))

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllFromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" \("address", "name"\) VALUES \('111 Test Addr', 'Test1'\) RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ds.Executor())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllRequiresStruct() {
	for _, ds := range []*goqu.SelectDataset{ss.db.From("items"), ss.db.From("items").Select("name")} {
		items, err := goqu.ScanAll[string](context.Background(), ds)
		ss.EqualError(err, "goqu: ScanAll requires a struct or a pointer to a struct, got string")
		ss.True(goqu.IsUnsupportedScanType(err))
		ss.Nil(items)
	}
}

func (ss *scanSuite) TestScanAllWithoutDatabase() {
	_, err := goqu.ScanAll[dsTestActionItem](context.Background(), goqu.From("items"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanOne() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOnePointer() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOne[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOnePointerNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := goqu.ScanOne[*dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.False(found)
	ss.Nil(item)
}

func (ss *scanSuite) TestScanOneNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnRows(ss.itemRows(""))

	item, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).WillReturnError(scanTestError{})

	_, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
}

func (ss *scanSuite) TestScanOneRowError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1").RowError(0, scanTestError{}))

	item, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, scanTestError{})
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneFromExecutor() {
	ss.mock.ExpectQuery(`UPDATE "items" SET "name"='Test2' RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test2"))

	ds := ss.db.Update("items").Set(goqu.Record{"name": "Test2"}).Returning("address", "name")
	item, found, err := goqu.ScanOne[*dsTestActionItem](context.Background(), ds.Executor())
	ss.NoError(err)
	ss.True(found)
	ss.Equal(&dsTestActionItem{Address: "111 Test Addr", Name: "Test2"}, item)
}

func (ss *scanSuite) TestScanOneRequiresStruct() {
	_, _, err := goqu.ScanOne[string](context.Background(), ss.db.From("items"))
	ss.EqualError(err, "goqu: ScanOne requires a struct or a pointer to a struct, got string")
	ss.True(goqu.IsUnsupportedScanType(err))

	_, _, err = goqu.ScanOne[**dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.True(goqu.IsUnsupportedScanType(err))
}

func (ss *scanSuite) TestScanOneWithoutDatabase() {
	_, _, err := goqu.ScanOne[dsTestActionItem](context.Background(), goqu.From("items"))
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

func (ss *scanSuite) TestScanOneValRejectsSlice() {
	_, _, err := goqu.ScanOneVal[[]int64](context.Background(), ss.db.From("items").Select("id"))
	ss.EqualError(err, "goqu: type cannot be a pointer to a slice when scanning into val")
}

func (ss *scanSuite) TestScanOneValWithoutDatabase() {
	_, _, err := goqu.ScanOneVal[int64](context.Background(), goqu.From("items").Select("id"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(scanSuite))
}
