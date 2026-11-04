package goqu_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/internal/errors"
)

type ctxKey struct{}

type ctxLogEntry struct {
	ctxValue interface{}
	message  string
}

type dbTestMockContextLogger struct {
	entries []ctxLogEntry
}

func (l *dbTestMockContextLogger) Printf(format string, v ...interface{}) {
	l.entries = append(l.entries, ctxLogEntry{ctxValue: "printf", message: fmt.Sprintf(format, v...)})
}

//nolint:goprintffuncname // implements goqu.ContextLogger
func (l *dbTestMockContextLogger) PrintfContext(ctx context.Context, format string, v ...interface{}) {
	l.entries = append(l.entries, ctxLogEntry{ctxValue: ctx.Value(ctxKey{}), message: fmt.Sprintf(format, v...)})
}

type beginTxRecordingDB struct {
	*sql.DB
	ctx  context.Context
	opts *sql.TxOptions
}

func (r *beginTxRecordingDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	r.ctx, r.opts = ctx, opts
	return r.DB.BeginTx(ctx, opts)
}

func (ds *databaseSuite) TestWithTxContext() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	db := goqu.New("mock", mDB)
	ctx := context.WithValue(context.Background(), ctxKey{}, "request-1")

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
				mock.ExpectBegin().WillReturnError(errors.New("transaction begin error"))
			},
			f:      func(_ *goqu.TxDatabase) error { return nil },
			errStr: "goqu: transaction begin error",
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			f:      func(_ *goqu.TxDatabase) error { return errors.New("transaction error") },
			errStr: "goqu: transaction error",
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback().WillReturnError(errors.New("transaction rollback error"))
			},
			f:      func(_ *goqu.TxDatabase) error { return errors.New("something wrong") },
			errStr: "goqu: transaction rollback error",
		},
		{
			expectf: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectCommit().WillReturnError(errors.New("commit error"))
			},
			f:      func(_ *goqu.TxDatabase) error { return nil },
			errStr: "goqu: commit error",
		},
	}
	for _, c := range cases {
		c.expectf(mock)
		err := db.WithTxContext(ctx, nil, c.f)
		if c.errStr != "" {
			ds.EqualError(err, c.errStr)
		} else {
			ds.NoError(err)
		}
	}
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestWithTxContext_beginsWithContextAndOptions() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	mock.ExpectCommit()
	recorder := &beginTxRecordingDB{DB: mDB}
	ctx := context.WithValue(context.Background(), ctxKey{}, "request-1")
	opts := &sql.TxOptions{ReadOnly: true}

	ds.NoError(goqu.New("mock", recorder).WithTxContext(ctx, opts, func(_ *goqu.TxDatabase) error { return nil }))
	ds.Equal("request-1", recorder.ctx.Value(ctxKey{}))
	ds.Same(opts, recorder.opts)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestWithTxContext_rollbackOnPanic() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	mock.ExpectRollback()

	ds.PanicsWithValue("a problem has happened", func() {
		_ = goqu.New("mock", mDB).WithTxContext(context.Background(), nil, func(_ *goqu.TxDatabase) error {
			panic("a problem has happened")
		})
	})
	ds.NoError(mock.ExpectationsWereMet())
}

type txEndedByContext struct {
	goqu.SQLTx
}

func (txEndedByContext) Rollback() error { return sql.ErrTxDone }

func (ds *databaseSuite) TestWrap_keepsErrorWhenTxAlreadyEnded() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tx := goqu.NewTx("mock", txEndedByContext{})
	err := tx.Wrap(func() error { return ctx.Err() })
	ds.ErrorIs(err, context.Canceled)
}

func (ds *databaseSuite) TestContextLogger_transactionBoundaries() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	db := goqu.New("mock", mDB)
	logger := new(dbTestMockContextLogger)
	db.Logger(logger)
	ctx := context.WithValue(context.Background(), ctxKey{}, "request-1")

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "items" SET "a"=1`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ds.NoError(db.WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		_, err := tx.ExecContext(ctx, `UPDATE "items" SET "a"=1`)
		return err
	}))

	mock.ExpectBegin()
	mock.ExpectRollback()
	ds.EqualError(db.WithTxContext(ctx, nil, func(_ *goqu.TxDatabase) error {
		return errors.New("transaction error")
	}), "goqu: transaction error")

	ds.Equal([]ctxLogEntry{
		{ctxValue: "request-1", message: "[goqu - transaction] BEGIN"},
		{ctxValue: "request-1", message: "[goqu - transaction] EXEC [query:=`UPDATE \"items\" SET \"a\"=1`] "},
		{ctxValue: "request-1", message: "[goqu - transaction] COMMIT"},
		{ctxValue: "request-1", message: "[goqu - transaction] BEGIN"},
		{ctxValue: "request-1", message: "[goqu - transaction] ROLLBACK"},
	}, logger.entries)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestContextLogger_withoutContext() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	db := goqu.New("mock", mDB)
	logger := new(dbTestMockContextLogger)
	db.Logger(logger)

	mock.ExpectExec(`UPDATE "items" SET "a"=2`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectCommit()
	_, err = db.Exec(`UPDATE "items" SET "a"=2`)
	ds.NoError(err)
	ds.NoError(db.WithTx(func(_ *goqu.TxDatabase) error { return nil }))

	ds.Equal([]ctxLogEntry{
		{ctxValue: nil, message: "[goqu] EXEC [query:=`UPDATE \"items\" SET \"a\"=2`]"},
		{ctxValue: nil, message: "[goqu - transaction] BEGIN"},
		{ctxValue: nil, message: "[goqu - transaction] COMMIT"},
	}, logger.entries)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestContextLogger_queryTraces() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	db := goqu.New("mock", mDB)
	logger := new(dbTestMockContextLogger)
	db.Logger(logger)
	ctx := context.WithValue(context.Background(), ctxKey{}, "request-1")
	query := `SELECT "a" FROM "items"`
	rows := func() *sqlmock.Rows { return sqlmock.NewRows([]string{"a"}).AddRow(1) }

	mock.ExpectExec(`UPDATE "items" SET "a"=1`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectPrepare(`SELECT "a" FROM "items"`)
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectBegin()
	mock.ExpectPrepare(`SELECT "a" FROM "items"`)
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectCommit()

	_, err = db.ExecContext(ctx, `UPDATE "items" SET "a"=1`)
	ds.NoError(err)
	_, err = db.PrepareContext(ctx, query)
	ds.NoError(err)
	r, err := db.QueryContext(ctx, query)
	ds.Require().NoError(err)
	ds.NoError(r.Err())
	ds.NoError(r.Close())
	var a int
	ds.NoError(db.QueryRowContext(ctx, query).Scan(&a))
	ds.NoError(db.WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		if _, err := tx.PrepareContext(ctx, query); err != nil {
			return err
		}
		r, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		if err := r.Err(); err != nil {
			return err
		}
		if err := r.Close(); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, query).Scan(&a)
	}))

	ops := make([]string, 0, len(logger.entries))
	for _, e := range logger.entries {
		ds.Equal("request-1", e.ctxValue, e.message)
		ops = append(ops, e.message)
	}
	ds.Equal([]string{
		"[goqu] EXEC [query:=`UPDATE \"items\" SET \"a\"=1`]",
		"[goqu] PREPARE [query:=`SELECT \"a\" FROM \"items\"`]",
		"[goqu] QUERY [query:=`SELECT \"a\" FROM \"items\"`]",
		"[goqu] QUERY ROW [query:=`SELECT \"a\" FROM \"items\"`]",
		"[goqu - transaction] BEGIN",
		"[goqu - transaction] PREPARE [query:=`SELECT \"a\" FROM \"items\"`] ",
		"[goqu - transaction] QUERY [query:=`SELECT \"a\" FROM \"items\"`] ",
		"[goqu - transaction] QUERY ROW [query:=`SELECT \"a\" FROM \"items\"`] ",
		"[goqu - transaction] COMMIT",
	}, ops)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestPrintfLogger_withTxContext() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	db := goqu.New("mock", mDB)
	logger := new(dbTestMockLogger)
	db.Logger(logger)

	mock.ExpectBegin()
	mock.ExpectCommit()
	ds.NoError(db.WithTxContext(context.Background(), nil, func(_ *goqu.TxDatabase) error { return nil }))
	ds.Equal([]string{
		"[goqu - transaction] BEGIN",
		"[goqu - transaction] COMMIT",
	}, logger.Messages)
	ds.NoError(mock.ExpectationsWereMet())
}
