/*
goqu an idiomatch SQL builder, and query package.

	  __ _  ___   __ _ _   _
	 / _` |/ _ \ / _` | | | |
	| (_| | (_) | (_| | |_| |
	 \__, |\___/ \__, |\__,_|
	 |___/          |_|

Please see https://github.com/doug-martin/goqu for an introduction to goqu.
*/
package goqu

import (
	"time"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/internal/util"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

type DialectWrapper struct {
	dialect string
	tag     tagRequest
}

// Creates a new DialectWrapper to create goqu.Datasets or goqu.Databases with the specified dialect.
func Dialect(dialect string) DialectWrapper {
	return DialectWithTagName(dialect, "")
}

func DialectWithTagName(dialect, requestedTag string) DialectWrapper {
	return DialectWrapper{dialect: dialect, tag: requestTag(requestedTag)}
}

func LookupDialect(dialect string) (DialectWrapper, error) {
	return LookupDialectWithTagName(dialect, "")
}

func LookupDialectWithTagName(dialect, requestedTag string) (DialectWrapper, error) {
	if _, ok := registeredDialect(dialect); !ok {
		return DialectWrapper{}, errors.NewUnknownDialectError(
			"dialect %q is not registered; blank-import its dialect package or call RegisterDialect", dialect,
		)
	}
	return DialectWithTagName(dialect, requestedTag), nil
}

// Create a new dataset for creating SELECT sql statements
func (dw DialectWrapper) From(table ...interface{}) *SelectDataset {
	return newDataset(dw.dialect, dw.tag, nil).From(table...)
}

// Create a new dataset for creating SELECT sql statements
func (dw DialectWrapper) Select(cols ...interface{}) *SelectDataset {
	return newDataset(dw.dialect, dw.tag, nil).Select(cols...)
}

// Create a new dataset for creating UPDATE sql statements
func (dw DialectWrapper) Update(table interface{}) *UpdateDataset {
	return newUpdateDataset(dw.dialect, dw.tag, nil).Table(table)
}

// Create a new dataset for creating INSERT sql statements
func (dw DialectWrapper) Insert(table interface{}) *InsertDataset {
	return newInsertDataset(dw.dialect, dw.tag, nil).Into(table)
}

// Create a new dataset for creating DELETE sql statements
func (dw DialectWrapper) Delete(table interface{}) *DeleteDataset {
	return newDeleteDataset(dw.dialect, dw.tag, nil).From(table)
}

// Create a new dataset for creating TRUNCATE sql statements
func (dw DialectWrapper) Truncate(table ...interface{}) *TruncateDataset {
	return newTruncateDataset(dw.dialect, dw.tag, nil).Table(table...)
}

func (dw DialectWrapper) ExpressionSQL(e exp.Expression) (sql string, args []interface{}, err error) {
	return dw.expressionSQL(e, false)
}

func (dw DialectWrapper) PreparedExpressionSQL(e exp.Expression) (sql string, args []interface{}, err error) {
	return dw.expressionSQL(e, true)
}

func (dw DialectWrapper) expressionSQL(e exp.Expression, prepared bool) (sql string, args []interface{}, err error) {
	b := sb.NewSQLBuilder(prepared)
	defer sb.ReleaseSQLBuilder(b)
	dialectFor(dw.dialect, dw.tag.resolve(dw.dialect)).toExpressionSQL(b, e)
	return b.ToSQL()
}

func (dw DialectWrapper) DB(db SQLDatabase) *Database {
	return newDatabase(dw.dialect, dw.tag, db)
}

func (dw DialectWrapper) Tx(tx SQLTx) *TxDatabase {
	return newTxDatabase(dw.dialect, dw.tag.resolve(dw.dialect), tx)
}

func New(dialect string, db SQLDatabase) *Database {
	return newDatabase(dialect, tagRequest{}, db)
}

func NewWithTagName(dialect, requestedTag string, db SQLDatabase) *Database {
	return newDatabase(dialect, requestTag(requestedTag), db)
}

// Set the behavior when encountering struct fields that do not have a db tag.
// By default this is false; if set to true any field without a db tag will not
// be targeted by Select or Scan operations.
func SetIgnoreUntaggedFields(ignore bool) {
	util.SetIgnoreUntaggedFields(ignore)
}

// Set the column rename function. This is used for struct fields that do not have a db tag to specify the column name
// By default all struct fields that do not have a db tag will be converted lowercase
func SetColumnRenameFunction(renameFunc func(string) string) {
	util.SetColumnRenameFunction(renameFunc)
}

// Set the location to use when interpolating time.Time instances. See https://golang.org/pkg/time/#LoadLocation
// NOTE: This has no effect when using prepared statements.
func SetTimeLocation(loc *time.Location) {
	sqlgen.SetTimeLocation(loc)
}
