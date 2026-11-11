package sqlgen

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlbuilder"
)

type (
	CommonSQLGenerator interface {
		TagName() string
		Dialect() string
		DialectOptions() *SQLDialectOptions
		ExpressionSQLGenerator() ExpressionSQLGenerator
		ReturningSQL(b sqlbuilder.SQLBuilder, returns exp.ColumnListExpression)
		ConflictTargetSQL(b sqlbuilder.SQLBuilder, targets exp.ColumnListExpression)
		FromSQL(b sqlbuilder.SQLBuilder, from exp.ColumnListExpression)
		SourcesSQL(b sqlbuilder.SQLBuilder, from exp.ColumnListExpression)
		WhereSQL(b sqlbuilder.SQLBuilder, where exp.ExpressionList)
		OrderSQL(b sqlbuilder.SQLBuilder, order exp.ColumnListExpression)
		OrderWithOffsetFetchSQL(b sqlbuilder.SQLBuilder, order exp.ColumnListExpression, offset uint, limit interface{})
		LimitSQL(b sqlbuilder.SQLBuilder, limit interface{})
		UpdateExpressionSQL(b sqlbuilder.SQLBuilder, updates ...exp.UpdateExpression)
	}
	commonSQLGenerator struct {
		dialect        string
		tagName        string
		esg            ExpressionSQLGenerator
		dialectOptions *SQLDialectOptions
	}
)

func NewCommonSQLGenerator(dialect, tagName string, do *SQLDialectOptions) CommonSQLGenerator {
	return &commonSQLGenerator{dialect: dialect, tagName: tagName, esg: NewExpressionSQLGenerator(dialect, tagName, do), dialectOptions: do}
}

func (csg *commonSQLGenerator) TagName() string {
	return csg.tagName
}

func (csg *commonSQLGenerator) Dialect() string {
	return csg.dialect
}

func (csg *commonSQLGenerator) DialectOptions() *SQLDialectOptions {
	return csg.dialectOptions
}

func (csg *commonSQLGenerator) ExpressionSQLGenerator() ExpressionSQLGenerator {
	return csg.esg
}

func (csg *commonSQLGenerator) ReturningSQL(b sqlbuilder.SQLBuilder, returns exp.ColumnListExpression) {
	if returns != nil && len(returns.Columns()) > 0 {
		if csg.dialectOptions.SupportsReturn {
			b.Write(csg.dialectOptions.ReturningFragment)
			csg.esg.Generate(b, returns)
		} else {
			b.SetError(errReturnNotSupported(csg.dialect))
		}
	}
}

func (csg *commonSQLGenerator) ConflictTargetSQL(b sqlbuilder.SQLBuilder, targets exp.ColumnListExpression) {
	if targets != nil && len(targets.Columns()) == 0 {
		b.SetError(errEmptyConflictTarget)
		return
	}
	if targets != nil {
		if csg.dialectOptions.SupportsConflictTarget {
			b.WriteRunes(csg.dialectOptions.SpaceRune)
			b.WriteRunes(csg.dialectOptions.LeftParenRune)
			csg.esg.Generate(b, targets)
			b.WriteRunes(csg.dialectOptions.RightParenRune)
		} else {
			b.SetError(errConflictTargetNotSupported(csg.dialect))
		}
	}
}

// Adds the FROM clause and tables to an sql statement
func (csg *commonSQLGenerator) FromSQL(b sqlbuilder.SQLBuilder, from exp.ColumnListExpression) {
	if from != nil && !from.IsEmpty() {
		b.Write(csg.dialectOptions.FromFragment)
		csg.SourcesSQL(b, from)
	}
}

// Adds the generates the SQL for a column list
func (csg *commonSQLGenerator) SourcesSQL(b sqlbuilder.SQLBuilder, from exp.ColumnListExpression) {
	b.WriteRunes(csg.dialectOptions.SpaceRune)
	csg.esg.Generate(b, from)
}

// Generates the WHERE clause for an SQL statement
func (csg *commonSQLGenerator) WhereSQL(b sqlbuilder.SQLBuilder, where exp.ExpressionList) {
	if where != nil {
		b.Write(csg.dialectOptions.WhereFragment)
		csg.esg.Generate(b, where)
	}
}

// Generates the ORDER BY clause for an SQL statement
func (csg *commonSQLGenerator) OrderSQL(b sqlbuilder.SQLBuilder, order exp.ColumnListExpression) {
	if order == nil {
		return
	}
	if len(order.Columns()) == 0 {
		b.SetError(errEmptyOrder)
		return
	}
	b.Write(csg.dialectOptions.OrderByFragment)
	csg.esg.Generate(b, order)
}

func (csg *commonSQLGenerator) OrderWithOffsetFetchSQL(
	b sqlbuilder.SQLBuilder,
	order exp.ColumnListExpression,
	offset uint,
	limit interface{},
) {
	if order == nil {
		if offset > 0 {
			b.SetError(errOffsetWithoutOrder(csg.dialect))
		}
		return
	}

	csg.OrderSQL(b, order)
	if offset > 0 {
		b.Write(csg.dialectOptions.OffsetFragment)
		csg.esg.Generate(b, offset)
		b.Write(csg.dialectOptions.OffsetRowsFragment)

		if isZeroLimit(limit) {
			b.SetError(errLimitZeroWithOffsetNotSupported(csg.dialect))
			return
		}
		if limit != nil && !isUnsupportedLimitAll(csg.dialectOptions, limit) {
			b.Write(csg.dialectOptions.FetchFragment)
			csg.esg.Generate(b, limit)
			b.Write(csg.dialectOptions.FetchRowsOnlyFragment)
		}
	}
}

func isZeroLimit(limit interface{}) bool {
	v := reflect.ValueOf(limit)
	switch {
	case v.CanInt():
		return v.Int() == 0
	case v.CanUint():
		return v.Uint() == 0
	default:
		return false
	}
}

func isLimitAll(limit interface{}) bool {
	l, ok := limit.(exp.LiteralExpression)
	return ok && l.Literal() == "ALL"
}

func isUnsupportedLimitAll(opts *SQLDialectOptions, limit interface{}) bool {
	return !opts.SupportsLimitAll && isLimitAll(limit)
}

func hasRealLimit(limit interface{}) bool {
	return limit != nil && !isLimitAll(limit)
}

// Generates the LIMIT clause for an SQL statement
func (csg *commonSQLGenerator) LimitSQL(b sqlbuilder.SQLBuilder, limit interface{}) {
	if limit != nil && !isUnsupportedLimitAll(csg.dialectOptions, limit) {
		b.Write(csg.dialectOptions.LimitFragment)
		if csg.dialectOptions.SurroundLimitWithParentheses {
			b.WriteRunes(csg.dialectOptions.LeftParenRune)
		}
		csg.esg.Generate(b, limit)
		if csg.dialectOptions.SurroundLimitWithParentheses {
			b.WriteRunes(csg.dialectOptions.RightParenRune)
		}
	}
}

func (csg *commonSQLGenerator) UpdateExpressionSQL(b sqlbuilder.SQLBuilder, updates ...exp.UpdateExpression) {
	if len(updates) == 0 {
		b.SetError(errNoUpdatedValuesProvided)
		return
	}
	updateLen := len(updates)
	for i, update := range updates {
		csg.esg.Generate(b, update)
		if i < updateLen-1 {
			b.WriteRunes(csg.dialectOptions.CommaRune)
		}
	}
}
