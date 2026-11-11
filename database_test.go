package goqu_test

import (
	"context"
	dbsql "database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/suite"
)

type testActionItem struct {
	Address string `db:"address"`
	Name    string `db:"name"`
}

type dbTestMockLogger struct {
	Messages []string
}

func (dtml *dbTestMockLogger) Printf(format string, v ...interface{}) {
	dtml.Messages = append(dtml.Messages, fmt.Sprintf(format, v...))
}

func (dtml *dbTestMockLogger) Reset() {
	dtml.Messages = dtml.Messages[0:0]
}

type databaseSuite struct {
	suite.Suite
}

func (ds *databaseSuite) TestLogger() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectExec(`SELECT \* FROM "items" WHERE "id" = ?`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 0))

	db := goqu.New("default", mDB)
	logger := new(dbTestMockLogger)
	db.Logger(logger)
	var items []testActionItem
	ds.NoError(db.ScanStructs(&items, `SELECT * FROM "items"`))
	_, err = db.Exec(`SELECT * FROM "items" WHERE "id" = ?`, 1)
	ds.NoError(err)
	db.Trace("TEST", "")
	ds.Equal([]string{
		"[goqu] QUERY [query:=`SELECT * FROM \"items\"`]",
		"[goqu] EXEC [query:=`SELECT * FROM \"items\" WHERE \"id\" = ?` args:=[1]]",
		"[goqu] TEST",
	}, logger.Messages)
}

func (ds *databaseSuite) TestScanStructs() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))
	mock.ExpectQuery(`SELECT "test" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("default", mDB)
	var items []testActionItem
	ds.NoError(db.ScanStructs(&items, `SELECT * FROM "items"`))
	ds.Len(items, 2)
	ds.Equal("111 Test Addr", items[0].Address)
	ds.Equal("Test1", items[0].Name)

	ds.Equal("211 Test Addr", items[1].Address)
	ds.Equal("Test2", items[1].Name)

	items = items[0:0]
	ds.EqualError(db.ScanStructs(items, `SELECT * FROM "items"`),
		"goqu: cannot scan into []goqu_test.testActionItem, a pointer to a slice is required")
	ds.EqualError(db.ScanStructs(&testActionItem{}, `SELECT * FROM "items"`),
		"goqu: cannot scan into *goqu_test.testActionItem, a pointer to a slice is required")
	ds.EqualError(db.ScanStructs(&items, `SELECT "test" FROM "items"`),
		`goqu: unable to find corresponding field to column "test" returned by query`)
}

func (ds *databaseSuite) TestScanStruct() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT \* FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	mock.ExpectQuery(`SELECT "test" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("default", mDB)
	var item testActionItem
	found, err := db.ScanStruct(&item, `SELECT * FROM "items" LIMIT 1`)
	ds.NoError(err)
	ds.True(found)
	ds.Equal("111 Test Addr", item.Address)
	ds.Equal("Test1", item.Name)

	_, err = db.ScanStruct(item, `SELECT * FROM "items" LIMIT 1`)
	ds.EqualError(err, "goqu: cannot scan into goqu_test.testActionItem, a pointer to a struct is required")
	_, err = db.ScanStruct([]testActionItem{}, `SELECT * FROM "items" LIMIT 1`)
	ds.EqualError(err, "goqu: cannot scan into []goqu_test.testActionItem, a pointer to a struct is required")
	_, err = db.ScanStruct(&item, `SELECT "test" FROM "items" LIMIT 1`)
	ds.EqualError(err, `goqu: unable to find corresponding field to column "test" returned by query`)
}

func (ds *databaseSuite) TestScanVals() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2\n3\n4\n5"))

	db := goqu.New("default", mDB)
	var ids []uint32
	ds.NoError(db.ScanVals(&ids, `SELECT "id" FROM "items"`))
	ds.Len(ids, 5)

	ds.EqualError(db.ScanVals([]uint32{}, `SELECT "id" FROM "items"`),
		"goqu: cannot scan into []uint32, a pointer to a slice is required")
	ds.EqualError(db.ScanVals(testActionItem{}, `SELECT "id" FROM "items"`),
		"goqu: cannot scan into goqu_test.testActionItem, a pointer to a slice is required")
}

func (ds *databaseSuite) TestScanVal() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("10"))

	db := goqu.New("default", mDB)
	var id int64
	found, err := db.ScanVal(&id, `SELECT "id" FROM "items"`)
	ds.NoError(err)
	ds.Equal(int64(10), id)
	ds.True(found)

	found, err = db.ScanVal([]int64{}, `SELECT "id" FROM "items"`)
	ds.False(found)
	ds.EqualError(err, "goqu: cannot scan into []int64, a pointer is required")
	found, err = db.ScanVal(10, `SELECT "id" FROM "items"`)
	ds.False(found)
	ds.EqualError(err, "goqu: cannot scan into int, a pointer is required")
}

func (ds *databaseSuite) TestFromSQL() {
	mDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	ds.Require().NoError(err)
	const itemsQuery = `SELECT "address", "name" FROM "items" WHERE "id" > ?`
	const countQuery = `SELECT COUNT(*) FROM "items" WHERE "id" > ?`
	itemRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2")
	}
	mock.ExpectQuery(itemsQuery).WithArgs(10).WillReturnRows(itemRows())
	mock.ExpectQuery(itemsQuery).WithArgs(10).WillReturnRows(itemRows())
	mock.ExpectQuery(countQuery).WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("2"))
	mock.ExpectQuery(countQuery).WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("2"))
	db := goqu.New("default", mDB)
	ctx := context.Background()

	items, err := db.FromSQL(itemsQuery, 10).ScanAllStructs[testActionItem](ctx)
	ds.NoError(err)
	ds.Equal([]testActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)

	item, found, err := db.FromSQL(itemsQuery, 10).ScanOneStruct[testActionItem](ctx)
	ds.NoError(err)
	ds.True(found)
	ds.Equal(testActionItem{Address: "111 Test Addr", Name: "Test1"}, item)

	counts, err := db.FromSQL(countQuery, 10).ScanAllVals[int64](ctx)
	ds.NoError(err)
	ds.Equal([]int64{2}, counts)

	count, found, err := db.FromSQL(countQuery, 10).ScanOneVal[int64](ctx)
	ds.NoError(err)
	ds.True(found)
	ds.Equal(int64(2), count)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestExpressionSQL() {
	db := goqu.New("mysql", nil)
	sql, args, err := db.ExpressionSQL(goqu.C("a").Eq(1))
	ds.NoError(err)
	ds.Equal("(`a` = 1)", sql)
	ds.Empty(args)

	sql, args, err = db.PreparedExpressionSQL(goqu.C("a").Eq(1))
	ds.NoError(err)
	ds.Equal("(`a` = ?)", sql)
	ds.Equal([]interface{}{int64(1)}, args)
}

func (ds *databaseSuite) TestExpressionSQL_usesDatabaseTag() {
	sql, _, err := goqu.NewWithTagName("default", "json", nil).ExpressionSQL(goqu.W().PartitionBy(taggedItem{}))
	ds.NoError(err)
	ds.Equal(`(PARTITION BY "json_id", "json_name")`, sql)
}

func (ds *databaseSuite) TestExec() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectExec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE \("name" IS NULL\)`).
		WithArgs().
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE \("name" IS NULL\)`).
		WithArgs().
		WillReturnError(newTestError("mock error"))

	db := goqu.New("default", mDB)
	_, err = db.Exec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE ("name" IS NULL)`)
	ds.NoError(err)
	_, err = db.Exec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE ("name" IS NULL)`)
	ds.EqualError(err, "mock error")
}

func (ds *databaseSuite) TestQuery() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnError(newTestError("mock error"))

	db := goqu.New("default", mDB)
	_, err = db.Query(`SELECT * FROM "items"`) //nolint:rowserrcheck // not checking row scan
	ds.NoError(err, "goqu - mock error")

	_, err = db.Query(`SELECT * FROM "items"`) //nolint:rowserrcheck // not checking row scan
	ds.EqualError(err, "mock error")
}

func (ds *databaseSuite) TestQueryRow() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnError(newTestError("mock error"))

	db := goqu.New("default", mDB)
	rows := db.QueryRow(`SELECT * FROM "items"`)
	var address string
	var name string
	ds.NoError(rows.Scan(&address, &name))

	rows = db.QueryRow(`SELECT * FROM "items"`)
	ds.EqualError(rows.Scan(&address, &name), "mock error")
}

func (ds *databaseSuite) TestPrepare() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectPrepare("SELECT \\* FROM test WHERE id = \\?")
	db := goqu.New("default", mDB)
	stmt, err := db.Prepare("SELECT * FROM test WHERE id = ?")
	ds.NoError(err)
	ds.NotNil(stmt)
}

func (ds *databaseSuite) TestBegin() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectBegin().WillReturnError(newTestError("transaction error"))
	db := goqu.New("default", mDB)
	logger := new(dbTestMockLogger)
	db.Logger(logger)
	tx, err := db.Begin()
	ds.NoError(err)
	ds.Equal("default", tx.Dialect())

	_, err = db.Begin()
	ds.EqualError(err, "transaction error")
	ds.Equal([]string{"[goqu - transaction] BEGIN", "[goqu - transaction] BEGIN"}, logger.Messages)
}

func (ds *databaseSuite) TestBeginTx() {
	ctx := context.Background()
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectBegin().WillReturnError(newTestError("transaction error"))
	db := goqu.New("default", mDB)
	logger := new(dbTestMockLogger)
	db.Logger(logger)
	tx, err := db.BeginTx(ctx, nil)
	ds.NoError(err)
	ds.Equal("default", tx.Dialect())

	_, err = db.BeginTx(ctx, nil)
	ds.EqualError(err, "transaction error")
	ds.Equal([]string{"[goqu - transaction] BEGIN", "[goqu - transaction] BEGIN"}, logger.Messages)
}

func (ds *databaseSuite) TestWithTx() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	ds.assertWithTxCases(mock, goqu.New("default", mDB).WithTx)
}

func (ds *databaseSuite) assertWithTxCases(mock sqlmock.Sqlmock, withTx func(func(*goqu.TxDatabase) error) error) {
	cases := []struct {
		expectf func(sqlmock.Sqlmock)
		f       func(*goqu.TxDatabase) error
		errStr  string
	}{
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit()
			},
			f: func(_ *goqu.TxDatabase) error { return nil },
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(newTestError("transaction begin error"))
			},
			f:      func(_ *goqu.TxDatabase) error { return nil },
			errStr: "transaction begin error",
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			f:      func(_ *goqu.TxDatabase) error { return newTestError("transaction error") },
			errStr: "transaction error",
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback().WillReturnError(newTestError("transaction rollback error"))
			},
			f:      func(_ *goqu.TxDatabase) error { return newTestError("something wrong") },
			errStr: "transaction rollback error",
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit().WillReturnError(newTestError("commit error"))
			},
			f:      func(_ *goqu.TxDatabase) error { return nil },
			errStr: "commit error",
		},
	}
	for _, c := range cases {
		c.expectf(mock)
		err := withTx(c.f)
		if c.errStr != "" {
			ds.EqualError(err, c.errStr)
		} else {
			ds.NoError(err)
		}
	}
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestRollbackOnPanic() {
	mDB, mock, err := sqlmock.New()

	defer func() {
		p := recover()
		if p == nil {
			ds.Fail("there should be a panic")
		}
		ds.Require().Equal("a problem has happened", p.(string))
		ds.Require().NoError(mock.ExpectationsWereMet())
	}()

	ds.NoError(err)

	mock.ExpectBegin()
	mock.ExpectRollback()

	db := goqu.New("default", mDB)
	_ = db.WithTx(func(_ *goqu.TxDatabase) error {
		panic("a problem has happened")
	})
}

func (ds *databaseSuite) TestDataRace() {
	mDB, mock, err := sqlmock.New()
	ds.NoError(err)
	db := goqu.New("default", mDB)

	const concurrency = 10

	for i := 0; i < concurrency; i++ {
		mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
			WithArgs().
			WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
				FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))
	}

	wg := sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			sql := db.From("items").Limit(1)
			var item testActionItem
			found, err := sql.ScanStruct(&item)
			ds.NoError(err)
			ds.True(found)
			ds.Equal(item.Address, "111 Test Addr")
			ds.Equal(item.Name, "Test1")
		}()
	}

	wg.Wait()
}

func TestDatabaseSuite(t *testing.T) {
	suite.Run(t, new(databaseSuite))
}

type txdatabaseSuite struct {
	suite.Suite
}

func (tds *txdatabaseSuite) TestLogger() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectExec(`SELECT \* FROM "items" WHERE "id" = ?`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	tx, err := goqu.New("default", mDB).Begin()
	tds.NoError(err)
	logger := new(dbTestMockLogger)
	tx.Logger(logger)
	var items []testActionItem
	tds.NoError(tx.ScanStructs(&items, `SELECT * FROM "items"`))
	_, err = tx.Exec(`SELECT * FROM "items" WHERE "id" = ?`, 1)
	tds.NoError(err)
	tds.NoError(tx.Commit())
	tds.Equal([]string{
		"[goqu - transaction] QUERY [query:=`SELECT * FROM \"items\"`] ",
		"[goqu - transaction] EXEC [query:=`SELECT * FROM \"items\" WHERE \"id\" = ?` args:=[1]] ",
		"[goqu - transaction] COMMIT",
	}, logger.Messages)
}

func (tds *txdatabaseSuite) TestLogger_FromDb() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectExec(`SELECT \* FROM "items" WHERE "id" = ?`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	db := goqu.New("default", mDB)
	logger := new(dbTestMockLogger)
	db.Logger(logger)
	tx, err := db.Begin()
	tds.NoError(err)

	var items []testActionItem
	tds.NoError(tx.ScanStructs(&items, `SELECT * FROM "items"`))
	_, err = tx.Exec(`SELECT * FROM "items" WHERE "id" = ?`, 1)
	tds.NoError(err)
	tds.NoError(tx.Commit())
	tds.Equal([]string{
		"[goqu - transaction] BEGIN",
		"[goqu - transaction] QUERY [query:=`SELECT * FROM \"items\"`] ",
		"[goqu - transaction] EXEC [query:=`SELECT * FROM \"items\" WHERE \"id\" = ?` args:=[1]] ",
		"[goqu - transaction] COMMIT",
	}, logger.Messages)
}

func (tds *txdatabaseSuite) TestCommit() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestRollback() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectRollback()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	tds.NoError(tx.Rollback())
}

func (tds *txdatabaseSuite) TestFrom() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	tds.NotNil(goqu.From("test"))
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestScanStructs() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))
	mock.ExpectQuery(`SELECT "test" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	var items []testActionItem
	tds.NoError(tx.ScanStructs(&items, `SELECT * FROM "items"`))
	tds.Len(items, 2)
	tds.Equal("111 Test Addr", items[0].Address)
	tds.Equal("Test1", items[0].Name)

	tds.Equal("211 Test Addr", items[1].Address)
	tds.Equal("Test2", items[1].Name)

	items = items[0:0]
	tds.EqualError(tx.ScanStructs(items, `SELECT * FROM "items"`),
		"goqu: cannot scan into []goqu_test.testActionItem, a pointer to a slice is required")
	tds.EqualError(tx.ScanStructs(&testActionItem{}, `SELECT * FROM "items"`),
		"goqu: cannot scan into *goqu_test.testActionItem, a pointer to a slice is required")
	tds.EqualError(tx.ScanStructs(&items, `SELECT "test" FROM "items"`),
		`goqu: unable to find corresponding field to column "test" returned by query`)
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestScanStruct() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	mock.ExpectQuery(`SELECT "test" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	var item testActionItem
	found, err := tx.ScanStruct(&item, `SELECT * FROM "items" LIMIT 1`)
	tds.NoError(err)
	tds.True(found)
	tds.Equal("111 Test Addr", item.Address)
	tds.Equal("Test1", item.Name)

	_, err = tx.ScanStruct(item, `SELECT * FROM "items" LIMIT 1`)
	tds.EqualError(err, "goqu: cannot scan into goqu_test.testActionItem, a pointer to a struct is required")
	_, err = tx.ScanStruct([]testActionItem{}, `SELECT * FROM "items" LIMIT 1`)
	tds.EqualError(err, "goqu: cannot scan into []goqu_test.testActionItem, a pointer to a struct is required")
	_, err = tx.ScanStruct(&item, `SELECT "test" FROM "items" LIMIT 1`)
	tds.EqualError(err, `goqu: unable to find corresponding field to column "test" returned by query`)
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestScanVals() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2\n3\n4\n5"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	var ids []uint32
	tds.NoError(tx.ScanVals(&ids, `SELECT "id" FROM "items"`))
	tds.Len(ids, 5)

	tds.EqualError(tx.ScanVals([]uint32{}, `SELECT "id" FROM "items"`),
		"goqu: cannot scan into []uint32, a pointer to a slice is required")
	tds.EqualError(tx.ScanVals(testActionItem{}, `SELECT "id" FROM "items"`),
		"goqu: cannot scan into goqu_test.testActionItem, a pointer to a slice is required")
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestScanVal() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("10"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	var id int64
	found, err := tx.ScanVal(&id, `SELECT "id" FROM "items"`)
	tds.NoError(err)
	tds.Equal(int64(10), id)
	tds.True(found)

	found, err = tx.ScanVal([]int64{}, `SELECT "id" FROM "items"`)
	tds.False(found)
	tds.EqualError(err, "goqu: cannot scan into []int64, a pointer is required")
	found, err = tx.ScanVal(10, `SELECT "id" FROM "items"`)
	tds.False(found)
	tds.EqualError(err, "goqu: cannot scan into int, a pointer is required")
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestFromSQL() {
	mDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	tds.Require().NoError(err)
	const query = `SELECT "address", "name" FROM "items" WHERE "id" > ?`
	mock.ExpectBegin()
	mock.ExpectQuery(query).WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))
	mock.ExpectCommit()
	tx, err := goqu.New("default", mDB).Begin()
	tds.Require().NoError(err)

	items, err := tx.FromSQL(query, 10).ScanAllStructs[testActionItem](context.Background())
	tds.NoError(err)
	tds.Equal([]testActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
	tds.NoError(tx.Commit())
	tds.NoError(mock.ExpectationsWereMet())
}

func (tds *txdatabaseSuite) TestExpressionSQL() {
	tx := goqu.NewTx("mysql", nil)
	sql, args, err := tx.ExpressionSQL(goqu.C("a").Eq(1))
	tds.NoError(err)
	tds.Equal("(`a` = 1)", sql)
	tds.Empty(args)

	sql, args, err = tx.PreparedExpressionSQL(goqu.C("a").Eq(1))
	tds.NoError(err)
	tds.Equal("(`a` = ?)", sql)
	tds.Equal([]interface{}{int64(1)}, args)
}

func (tds *txdatabaseSuite) TestExpressionSQL_usesDatabaseTag() {
	sql, _, err := goqu.NewTxWithTagName("default", "json", nil).ExpressionSQL(goqu.W().PartitionBy(taggedItem{}))
	tds.NoError(err)
	tds.Equal(`(PARTITION BY "json_id", "json_name")`, sql)
}

func (tds *txdatabaseSuite) TestExec() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE \("name" IS NULL\)`).
		WithArgs().
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE \("name" IS NULL\)`).
		WithArgs().
		WillReturnError(newTestError("mock error"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	_, err = tx.Exec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE ("name" IS NULL)`)
	tds.NoError(err)
	_, err = tx.Exec(`UPDATE "items" SET "address"='111 Test Addr',"name"='Test1' WHERE ("name" IS NULL)`)
	tds.EqualError(err, "mock error")
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestQuery() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnError(newTestError("mock error"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	_, err = tx.Query(`SELECT * FROM "items"`) //nolint:rowserrcheck // not checking row scan
	tds.NoError(err, "goqu - mock error")

	_, err = tx.Query(`SELECT * FROM "items"`) //nolint:rowserrcheck // not checking row scan
	tds.EqualError(err, "mock error")
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestQueryRow() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	mock.ExpectQuery(`SELECT \* FROM "items"`).
		WithArgs().
		WillReturnError(newTestError("mock error"))
	mock.ExpectCommit()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	rows := tx.QueryRow(`SELECT * FROM "items"`)
	var address string
	var name string
	tds.NoError(rows.Scan(&address, &name))

	rows = tx.QueryRow(`SELECT * FROM "items"`)
	tds.EqualError(rows.Scan(&address, &name), "mock error")
	tds.NoError(tx.Commit())
}

func (tds *txdatabaseSuite) TestWrap() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectRollback()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)
	tds.NoError(tx.Wrap(func() error {
		return nil
	}))
	tx, err = db.Begin()
	tds.NoError(err)
	tds.EqualError(tx.Wrap(func() error {
		return newTestError("tx error")
	}), "tx error")
}

func (tds *txdatabaseSuite) TestWrap_joinsErrorWhenTxAlreadyDone() {
	mDB, mock, err := sqlmock.New()
	tds.Require().NoError(err)
	mock.ExpectBegin()
	mock.ExpectRollback()
	tx, err := goqu.New("default", mDB).Begin()
	tds.Require().NoError(err)
	fnErr := newTestError("fn failed")

	err = tx.Wrap(func() error {
		tds.NoError(tx.Rollback())
		return fnErr
	})
	tds.ErrorIs(err, fnErr)
	tds.ErrorIs(err, dbsql.ErrTxDone)
	tds.NoError(mock.ExpectationsWereMet())
}

func (tds *txdatabaseSuite) TestWrap_doesNotRepeatErrTxDone() {
	mDB, mock, err := sqlmock.New()
	tds.Require().NoError(err)
	mock.ExpectBegin()
	mock.ExpectRollback()
	tx, err := goqu.New("default", mDB).Begin()
	tds.Require().NoError(err)

	err = tx.Wrap(func() error {
		tds.NoError(tx.Rollback())
		_, execErr := tx.Exec(`UPDATE "items" SET "a"=1`)
		return execErr
	})
	tds.Equal(dbsql.ErrTxDone, err)
	tds.NoError(mock.ExpectationsWereMet())
}

func (tds *txdatabaseSuite) TestWrap_returnsErrTxDoneWhenFnCommitted() {
	mDB, mock, err := sqlmock.New()
	tds.Require().NoError(err)
	mock.ExpectBegin()
	mock.ExpectCommit()
	tx, err := goqu.New("default", mDB).Begin()
	tds.Require().NoError(err)

	err = tx.Wrap(tx.Commit)
	tds.Equal(dbsql.ErrTxDone, err)
	tds.NoError(mock.ExpectationsWereMet())
}

func (tds *txdatabaseSuite) TestDataRace() {
	mDB, mock, err := sqlmock.New()
	tds.NoError(err)
	mock.ExpectBegin()
	db := goqu.New("default", mDB)
	tx, err := db.Begin()
	tds.NoError(err)

	const concurrency = 10

	for i := 0; i < concurrency; i++ {
		mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
			WithArgs().
			WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
				FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))
	}

	wg := sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			sql := tx.From("items").Limit(1)
			var item testActionItem
			found, err := sql.ScanStruct(&item)
			tds.NoError(err)
			tds.True(found)
			tds.Equal(item.Address, "111 Test Addr")
			tds.Equal(item.Name, "Test1")
		}()
	}

	wg.Wait()
	mock.ExpectCommit()
	tds.NoError(tx.Commit())
}

func TestTxDatabaseSuite(t *testing.T) {
	suite.Run(t, new(txdatabaseSuite))
}
