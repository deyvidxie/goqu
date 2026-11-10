package sqlgen

import (
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
)

type (
	// An adapter interface to be used by a Dataset to generate SQL for a specific dialect.
	// See DefaultAdapter for a concrete implementation and examples.
	UpdateSQLGenerator interface {
		Dialect() string
		Generate(b sb.SQLBuilder, clauses exp.UpdateClauses)
	}
	// The default adapter. This class should be used when building a new adapter. When creating a new adapter you can
	// either override methods, or more typically update default values.
	// See (github.com/doug-martin/goqu/dialect/postgres)
	updateSQLGenerator struct {
		CommonSQLGenerator
	}
)

func NewUpdateSQLGenerator(dialect, tagName string, do *SQLDialectOptions) UpdateSQLGenerator {
	return &updateSQLGenerator{NewCommonSQLGenerator(dialect, tagName, do)}
}

func (usg *updateSQLGenerator) Generate(b sb.SQLBuilder, clauses exp.UpdateClauses) {
	if !clauses.HasTable() {
		b.SetError(errNoSourceForUpdate)
		return
	}
	if !clauses.HasSetValues() {
		b.SetError(errNoSetValuesForUpdate)
		return
	}
	if !usg.DialectOptions().SupportsMultipleUpdateTables && clauses.HasFrom() {
		b.SetError(errMultipleUpdateTablesNotSupported(usg.Dialect()))
	}
	updates, err := exp.NewUpdateExpressions(usg.TagName(), clauses.SetValues())
	if err != nil {
		b.SetError(err)
		return
	}
	for _, f := range usg.DialectOptions().UpdateSQLOrder {
		if b.Error() != nil {
			return
		}
		switch f {
		case CommonTableSQLFragment:
			usg.ExpressionSQLGenerator().Generate(b, clauses.CommonTables())
		case UpdateBeginSQLFragment:
			usg.UpdateBeginSQL(b)
		case SourcesSQLFragment:
			usg.updateTableSQL(b, clauses)
		case UpdateSQLFragment:
			usg.UpdateExpressionsSQL(b, updates...)
		case UpdateFromSQLFragment:
			usg.updateFromSQL(b, clauses.From())
		case WhereSQLFragment:
			if w := clauses.Where(); w != nil && w.IsEmpty() {
				b.SetError(errEmptyWhereForUpdate)
				return
			}
			usg.WhereSQL(b, clauses.Where())
		case OrderSQLFragment:
			usg.updateOrderSQL(b, clauses)
		case LimitSQLFragment:
			usg.updateLimitSQL(b, clauses)
		case ReturningSQLFragment:
			usg.ReturningSQL(b, clauses.Returning())
		default:
			b.SetError(errFragmentNotSupported(usg.Dialect(), "UPDATE", f))
		}
	}
}

func (usg *updateSQLGenerator) updateLimitSQL(b sb.SQLBuilder, clauses exp.UpdateClauses) {
	switch {
	case usg.DialectOptions().SupportsLimitOnUpdate:
		usg.LimitSQL(b, clauses.Limit())
	case hasRealLimit(clauses.Limit()) && usg.DialectOptions().ErrorOnUnsupportedLimitOnUpdate:
		b.SetError(errLimitOnUpdateNotSupported(usg.Dialect()))
	}
}

func (usg *updateSQLGenerator) updateOrderSQL(b sb.SQLBuilder, clauses exp.UpdateClauses) {
	switch {
	case usg.DialectOptions().SupportsOrderByOnUpdate:
		usg.OrderSQL(b, clauses.Order())
	case clauses.HasOrder() && usg.DialectOptions().ErrorOnUnsupportedOrderByOnUpdate:
		b.SetError(errOrderByOnUpdateNotSupported(usg.Dialect()))
	}
}

// Adds the correct fragment to being an UPDATE statement
func (usg *updateSQLGenerator) UpdateBeginSQL(b sb.SQLBuilder) {
	b.Write(usg.DialectOptions().UpdateClause)
}

// Adds column setters in an update SET clause
func (usg *updateSQLGenerator) UpdateExpressionsSQL(b sb.SQLBuilder, updates ...exp.UpdateExpression) {
	b.Write(usg.DialectOptions().SetFragment)
	usg.UpdateExpressionSQL(b, updates...)
}

func (usg *updateSQLGenerator) updateTableSQL(b sb.SQLBuilder, uc exp.UpdateClauses) {
	b.WriteRunes(usg.DialectOptions().SpaceRune)
	usg.ExpressionSQLGenerator().Generate(b, uc.Table())
	if uc.HasFrom() {
		if !usg.DialectOptions().UseFromClauseForMultipleUpdateTables {
			b.WriteRunes(usg.DialectOptions().CommaRune)
			usg.ExpressionSQLGenerator().Generate(b, uc.From())
		}
	}
}

func (usg *updateSQLGenerator) updateFromSQL(b sb.SQLBuilder, ce exp.ColumnListExpression) {
	if ce == nil || ce.IsEmpty() {
		return
	}
	if usg.DialectOptions().UseFromClauseForMultipleUpdateTables {
		usg.FromSQL(b, ce)
	}
}
