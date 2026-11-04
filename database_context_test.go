package goqu_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

type ctxKey struct{}

type ctxLogEntry struct {
	ctxValue interface{}
	message  string
}

type dbTestMockContextLogger struct {
	entries []ctxLogEntry
}

func (l *dbTestMockContextLogger) Printf(format string, _ ...interface{}) {
	panic("Printf called on a ContextLogger: " + format)
}

//nolint:goprintffuncname
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
	ds.assertWithTxCases(mock, func(f func(*goqu.TxDatabase) error) error {
		return db.WithTxContext(context.Background(), nil, f)
	})
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

func (ds *databaseSuite) awaitRollbackByDatabaseSQL(tx *goqu.TxDatabase) {
	ds.EventuallyWithT(func(c *assert.CollectT) {
		_, err := tx.Tx.ExecContext(context.Background(), "SELECT 1")
		assert.ErrorIs(c, err, sql.ErrTxDone)
	}, time.Second, time.Millisecond)
}

func (ds *databaseSuite) TestWithTxContext_keepsCancelCauseOnCommit() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("request aborted")

	err = goqu.New("mock", mDB).WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		cancel(cause)
		ds.awaitRollbackByDatabaseSQL(tx)
		return nil
	})
	ds.ErrorIs(err, sql.ErrTxDone)
	ds.ErrorIs(err, cause)
	ds.ErrorIs(err, context.Canceled)
	ds.NoError(mock.ExpectationsWereMet())
}

type commitErrTx struct {
	goqu.SQLTx
	err error
}

func (t commitErrTx) Commit() error { return t.err }

func (ds *databaseSuite) TestWithTxContext_keepsCancelCauseWhenCommitSeesCancel() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("request aborted")

	err = goqu.New("mock", mDB).WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		tx.Tx = commitErrTx{err: context.Canceled}
		cancel(cause)
		return nil
	})
	ds.ErrorIs(err, context.Canceled)
	ds.ErrorIs(err, cause)
	ds.NotErrorIs(err, sql.ErrTxDone)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestWithTxContext_doesNotJoinCauseIntoUnrelatedError() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	ctx, cancel := context.WithCancelCause(context.Background())
	constraintErr := errors.New("unique constraint violated")

	err = goqu.New("mock", mDB).WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		tx.Tx = commitErrTx{err: constraintErr}
		cancel(errors.New("request aborted"))
		return nil
	})
	ds.Equal(constraintErr, err)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestWithTxContext_keepsCancelCauseOnRollback() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("request aborted")

	err = goqu.New("mock", mDB).WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		cancel(cause)
		ds.awaitRollbackByDatabaseSQL(tx)
		_, execErr := tx.ExecContext(ctx, `UPDATE "items" SET "a"=1`)
		return execErr
	})
	ds.ErrorIs(err, context.Canceled)
	ds.ErrorIs(err, sql.ErrTxDone)
	ds.ErrorIs(err, cause)
	ds.NoError(mock.ExpectationsWereMet())
}

func (ds *databaseSuite) TestWithTxContext_plainCancelIsNotJoinedTwice() {
	mDB, mock, err := sqlmock.New()
	ds.Require().NoError(err)
	mock.ExpectBegin()
	ctx, cancel := context.WithCancel(context.Background())

	err = goqu.New("mock", mDB).WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		cancel()
		ds.awaitRollbackByDatabaseSQL(tx)
		return ctx.Err()
	})
	ds.ErrorIs(err, sql.ErrTxDone)
	ds.Equal(1, strings.Count(err.Error(), context.Canceled.Error()))
	ds.NoError(mock.ExpectationsWereMet())
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
	db.Trace("TEST", "")
	ds.NoError(db.WithTx(func(tx *goqu.TxDatabase) error {
		tx.Trace("TEST", "")
		return nil
	}))

	ds.Equal([]ctxLogEntry{
		{ctxValue: nil, message: "[goqu] EXEC [query:=`UPDATE \"items\" SET \"a\"=2`]"},
		{ctxValue: nil, message: "[goqu] TEST"},
		{ctxValue: nil, message: "[goqu - transaction] BEGIN"},
		{ctxValue: nil, message: "[goqu - transaction] TEST"},
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

	mock.ExpectExec(`UPDATE "items" SET "a"=\?`).WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectPrepare(`SELECT "a" FROM "items"`)
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "items" SET "a"=\?`).WithArgs(2).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectPrepare(`SELECT "a" FROM "items"`)
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectQuery(`SELECT "a" FROM "items"`).WillReturnRows(rows())
	mock.ExpectCommit()

	_, err = db.ExecContext(ctx, `UPDATE "items" SET "a"=?`, 1)
	ds.NoError(err)
	_, err = db.PrepareContext(ctx, query)
	ds.NoError(err)
	r, err := db.QueryContext(ctx, query)
	ds.Require().NoError(err)
	ds.NoError(r.Err())
	ds.NoError(r.Close())
	var a int
	ds.NoError(db.QueryRowContext(ctx, query).Scan(&a))
	db.TraceContext(ctx, "TEST", "")
	ds.NoError(db.WithTxContext(ctx, nil, func(tx *goqu.TxDatabase) error {
		if _, err := tx.ExecContext(ctx, `UPDATE "items" SET "a"=?`, 2); err != nil {
			return err
		}
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
		"[goqu] EXEC [query:=`UPDATE \"items\" SET \"a\"=?` args:=[1]]",
		"[goqu] PREPARE [query:=`SELECT \"a\" FROM \"items\"`]",
		"[goqu] QUERY [query:=`SELECT \"a\" FROM \"items\"`]",
		"[goqu] QUERY ROW [query:=`SELECT \"a\" FROM \"items\"`]",
		"[goqu] TEST",
		"[goqu - transaction] BEGIN",
		"[goqu - transaction] EXEC [query:=`UPDATE \"items\" SET \"a\"=?` args:=[2]] ",
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
