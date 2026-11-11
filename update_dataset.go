package goqu

import (
	"github.com/doug-martin/goqu/v9/exec"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
)

type UpdateDataset struct {
	dialect      SQLDialect
	clauses      exp.UpdateClauses
	isPrepared   prepared
	queryFactory exec.QueryFactory
	tagName      string
	requestedTag tagRequest
	err          error
}

func newUpdateDataset(d string, requestedTag tagRequest, queryFactory exec.QueryFactory) *UpdateDataset {
	tagName := requestedTag.resolve(d)
	return &UpdateDataset{
		clauses:      exp.NewUpdateClauses(),
		dialect:      dialectFor(d, tagName),
		queryFactory: queryFactory,
		tagName:      tagName,
		requestedTag: requestedTag,
	}
}

func Update(table interface{}) *UpdateDataset {
	return newUpdateDataset("default", tagRequest{}, nil).Table(table)
}

// Set the parameter interpolation behavior. See examples
//
// prepared: If true the dataset WILL NOT interpolate the parameters.
func (ud *UpdateDataset) Prepared(prepared bool) *UpdateDataset {
	ret := ud.copy(ud.clauses)
	ret.isPrepared = preparedFromBool(prepared)
	return ret
}

func (ud *UpdateDataset) IsPrepared() bool {
	return ud.isPrepared.Bool()
}

// Sets the adapter used to serialize values and create the SQL statement
func (ud *UpdateDataset) WithDialect(dl string) *UpdateDataset {
	ds := ud.copy(ud.GetClauses())
	ds.tagName = ds.requestedTag.resolve(dl)
	ds.dialect = dialectFor(dl, ds.tagName)
	return ds
}

// Returns the current adapter on the dataset
func (ud *UpdateDataset) Dialect() SQLDialect {
	return ud.dialect
}

// Returns the current adapter on the dataset
func (ud *UpdateDataset) SetDialect(dialect SQLDialect) *UpdateDataset {
	cd := ud.copy(ud.GetClauses())
	cd.dialect = dialect
	return cd
}

func (ud *UpdateDataset) Expression() exp.Expression {
	return ud
}

// Clones the dataset
func (ud *UpdateDataset) Clone() exp.Expression {
	return ud.copy(ud.clauses)
}

// Returns the current clauses on the dataset.
func (ud *UpdateDataset) GetClauses() exp.UpdateClauses {
	return ud.clauses
}

// used internally to copy the dataset
func (ud *UpdateDataset) copy(clauses exp.UpdateClauses) *UpdateDataset {
	return &UpdateDataset{
		dialect:      ud.dialect,
		clauses:      clauses,
		isPrepared:   ud.isPrepared,
		queryFactory: ud.queryFactory,
		tagName:      ud.tagName,
		requestedTag: ud.requestedTag,
		err:          ud.err,
	}
}

// Creates a WITH clause for a common table expression (CTE).
//
// The name will be available to use in the UPDATE from in the associated query; and can optionally
// contain a list of column names "name(col1, col2, col3)".
//
// The name will refer to the results of the specified subquery.
func (ud *UpdateDataset) With(name string, subquery exp.Expression) *UpdateDataset {
	return ud.copy(ud.clauses.CommonTablesAppend(exp.NewCommonTableExpression(false, name, subquery)))
}

// Creates a WITH RECURSIVE clause for a common table expression (CTE)
//
// The name will be available to use in the UPDATE from in the associated query; and must
// contain a list of column names "name(col1, col2, col3)" for a recursive clause.
//
// The name will refer to the results of the specified subquery. The subquery for
// a recursive query will always end with a UNION or UNION ALL with a clause that
// refers to the CTE by name.
func (ud *UpdateDataset) WithRecursive(name string, subquery exp.Expression) *UpdateDataset {
	return ud.copy(ud.clauses.CommonTablesAppend(exp.NewCommonTableExpression(true, name, subquery)))
}

// Sets the table to update.
func (ud *UpdateDataset) Table(table interface{}) *UpdateDataset {
	switch t := table.(type) {
	case exp.Expression:
		return ud.copy(ud.clauses.SetTable(t))
	case string:
		return ud.copy(ud.clauses.SetTable(exp.ParseIdentifier(t)))
	default:
		panic(errUnsupportedTableType(table))
	}
}

// Sets the values to use in the SET clause. See examples.
func (ud *UpdateDataset) Set(values interface{}) *UpdateDataset {
	return ud.copy(ud.clauses.SetSetValues(values))
}

// Allows specifying other tables to reference in your update (If your dialect supports it). See examples.
func (ud *UpdateDataset) From(tables ...interface{}) *UpdateDataset {
	return ud.copy(ud.clauses.SetFrom(exp.NewColumnListExpression(nil, ud.tagName, tables...)))
}

// Adds a WHERE clause. See examples.
func (ud *UpdateDataset) Where(expressions ...exp.Expression) *UpdateDataset {
	if len(expressions) == 0 {
		return ud.copy(ud.clauses).SetError(errEmptyWhere)
	}
	return ud.copy(ud.clauses.WhereAppend(expressions...))
}

// Removes the WHERE clause. See examples.
func (ud *UpdateDataset) ClearWhere() *UpdateDataset {
	return ud.copy(ud.clauses.ClearWhere())
}

// Adds a ORDER clause. If the ORDER is currently set it replaces it. See examples.
func (ud *UpdateDataset) Order(order ...exp.OrderedExpression) *UpdateDataset {
	if err := checkOrder(order, errEmptyOrder); err != nil {
		return ud.copy(ud.clauses).SetError(err)
	}
	return ud.copy(ud.clauses.SetOrder(order...))
}

// Adds a more columns to the current ORDER BY clause. If no order has be previously specified it is the same as
// calling Order. See examples.
func (ud *UpdateDataset) OrderAppend(order ...exp.OrderedExpression) *UpdateDataset {
	if err := checkOrder(order, errEmptyOrderAppend); err != nil {
		return ud.copy(ud.clauses).SetError(err)
	}
	return ud.copy(ud.clauses.OrderAppend(order...))
}

// Adds a more columns to the beginning of the current ORDER BY clause. If no order has be previously specified it is the same as
// calling Order. See examples.
func (ud *UpdateDataset) OrderPrepend(order ...exp.OrderedExpression) *UpdateDataset {
	if err := checkOrder(order, errEmptyOrderPrepend); err != nil {
		return ud.copy(ud.clauses).SetError(err)
	}
	return ud.copy(ud.clauses.OrderPrepend(order...))
}

// Removes the ORDER BY clause. See examples.
func (ud *UpdateDataset) ClearOrder() *UpdateDataset {
	return ud.copy(ud.clauses.ClearOrder())
}

// Adds a LIMIT clause. If the LIMIT is currently set it replaces it. See examples.
func (ud *UpdateDataset) Limit(limit uint) *UpdateDataset {
	return ud.copy(ud.clauses.SetLimit(limit))
}

// Replaces any LIMIT with LIMIT ALL; a dialect that cannot render it renders no limit. See examples.
func (ud *UpdateDataset) LimitAll() *UpdateDataset {
	return ud.copy(ud.clauses.SetLimit(L("ALL")))
}

// Removes the LIMIT clause.
func (ud *UpdateDataset) ClearLimit() *UpdateDataset {
	return ud.copy(ud.clauses.ClearLimit())
}

// Adds a RETURNING clause to the dataset if the adapter supports it. See examples.
func (ud *UpdateDataset) Returning(returning ...interface{}) *UpdateDataset {
	return ud.copy(ud.clauses.SetReturning(exp.NewColumnListExpression(nil, ud.tagName, returning...)))
}

// Get any error that has been set or nil if no error has been set.
func (ud *UpdateDataset) Error() error {
	return ud.err
}

// Set an error on the dataset if one has not already been set. This error will be returned by a future call to Error
// or as part of ToSQL. This can be used by end users to record errors while building up queries without having to
// track those separately.
func (ud *UpdateDataset) SetError(err error) *UpdateDataset {
	if ud.err == nil {
		ud.err = err
	}

	return ud
}

// Generates an UPDATE sql statement, if Prepared has been called with true then the parameters will not be interpolated.
// See examples.
//
// Errors:
//   - There is an error generating the SQL
func (ud *UpdateDataset) ToSQL() (sql string, params []interface{}, err error) {
	builder := ud.updateSQLBuilder()
	defer sb.ReleaseSQLBuilder(builder)
	return builder.ToSQL()
}

// Appends this Dataset's UPDATE statement to the SQLBuilder
// This is used internally when using updates in CTEs
func (ud *UpdateDataset) AppendSQL(b sb.SQLBuilder) {
	if ud.err != nil {
		b.SetError(ud.err)
		return
	}
	ud.dialect.ToUpdateSQL(b, ud.GetClauses())
}

func (ud *UpdateDataset) GetAs() exp.IdentifierExpression {
	return nil
}

func (ud *UpdateDataset) ReturnsColumns() bool {
	return ud.clauses.HasReturning()
}

// Generates the UPDATE sql, and returns an exec.QueryExecutor with the sql set to the UPDATE statement
//
//	db.Update("test").Set(Record{"name":"Bob", update: time.Now()}).Executor()
func (ud *UpdateDataset) Executor() exec.QueryExecutor {
	if ud.queryFactory == nil {
		return noDatabaseExecutor(ud.tagName)
	}
	builder := ud.updateSQLBuilder()
	defer sb.ReleaseSQLBuilder(builder)
	return ud.queryFactory.FromSQLBuilder(builder)
}

func (ud *UpdateDataset) updateSQLBuilder() sb.SQLBuilder {
	buf := sb.NewSQLBuilder(ud.isPrepared.Bool())
	if ud.err != nil {
		return buf.SetError(ud.err)
	}
	ud.dialect.ToUpdateSQL(buf, ud.clauses)
	return buf
}
