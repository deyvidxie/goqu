package goqu_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/suite"
)

type scanSuite struct {
	suite.Suite
	db   *goqu.Database
	mock sqlmock.Sqlmock
}

var errScanQuery = errors.New("query failed")

func (ss *scanSuite) SetupTest() {
	mDB, mock, err := sqlmock.New()
	ss.Require().NoError(err)
	ss.db = goqu.New("mock", mDB)
	ss.mock = mock
}

func (ss *scanSuite) TearDownTest() {
	ss.NoError(ss.mock.ExpectationsWereMet())
}

func (ss *scanSuite) TestScanAll() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestScanAllNoRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}))

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2").
			RowError(1, errScanQuery))

	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, errScanQuery)
	ss.Nil(items)
}

func (ss *scanSuite) TestScanAllFromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" \("address", "name"\) VALUES \('111 Test Addr', 'Test1'\) RETURNING "address", "name"`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, err := goqu.ScanAll[dsTestActionItem](context.Background(), ds.Executor())
	ss.NoError(err)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestScanAllRequiresStruct() {
	ss.mock.ExpectQuery(`SELECT "name" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).FromCSVString("Test1"))

	_, err := goqu.ScanAll[string](context.Background(), ss.db.From("items").Select("name"))
	ss.EqualError(err, "goqu: cannot scan into this type: string")
}

func (ss *scanSuite) TestScanAllQueryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(errScanQuery)

	_, err := goqu.ScanAll[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, errScanQuery)
}

func (ss *scanSuite) TestScanAllWithoutDatabase() {
	_, err := goqu.ScanAll[dsTestActionItem](context.Background(), goqu.From("items"))
	ss.Equal(goqu.ErrQueryFactoryNotFoundError, err)
}

func (ss *scanSuite) TestScanOne() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	item, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}, item)
}

func (ss *scanSuite) TestScanOneNotFound() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}))

	item, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.NoError(err)
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanOneError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1").
			RowError(0, errScanQuery))

	item, found, err := goqu.ScanOne[dsTestActionItem](context.Background(), ss.db.From("items"))
	ss.ErrorIs(err, errScanQuery)
	ss.False(found)
	ss.Zero(item)
}

func (ss *scanSuite) TestScanVals() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ids, err := goqu.ScanVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestScanValsNoRows() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ids, err := goqu.ScanVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanValsError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2").RowError(1, errScanQuery))

	ids, err := goqu.ScanVals[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.ErrorIs(err, errScanQuery)
	ss.Nil(ids)
}

func (ss *scanSuite) TestScanVal() {
	ss.mock.ExpectQuery(`SELECT COUNT\(\*\) AS "count" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("10"))

	count, found, err := goqu.ScanVal[int64](context.Background(), ss.db.From("items").Select(goqu.COUNT("*").As("count")))
	ss.NoError(err)
	ss.True(found)
	ss.Equal(int64(10), count)
}

func (ss *scanSuite) TestScanValNotFound() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	id, found, err := goqu.ScanVal[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.NoError(err)
	ss.False(found)
	ss.Zero(id)
}

func (ss *scanSuite) TestScanValError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1").RowError(0, errScanQuery))

	id, found, err := goqu.ScanVal[int64](context.Background(), ss.db.From("items").Select("id"))
	ss.ErrorIs(err, errScanQuery)
	ss.False(found)
	ss.Zero(id)
}

func TestScanSuite(t *testing.T) {
	suite.Run(t, new(scanSuite))
}
