package sqlgen

import (
	"strings"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/doug-martin/goqu/v9/internal/sb"
)

type (
	// An adapter interface to be used by a Dataset to generate SQL for a specific dialect.
	// See DefaultAdapter for a concrete implementation and examples.
	InsertSQLGenerator interface {
		Dialect() string
		Generate(b sb.SQLBuilder, clauses exp.InsertClauses)
	}
	// The default adapter. This class should be used when building a new adapter. When creating a new adapter you can
	// either override methods, or more typically update default values.
	// See (github.com/doug-martin/goqu/dialect/postgres)
	insertSQLGenerator struct {
		CommonSQLGenerator
	}
	conflictUpdateBuilder struct {
		sb.SQLBuilder
		rowAlias     exp.IdentifierExpression
		usesExcluded bool
	}
)

var (
	selectAllFromFragment = []byte(" SELECT * FROM (")

	ErrConflictUpdateValuesRequired = errors.New("values are required for on conflict update expression")
	ErrNoSourceForInsert            = errors.New("no source found when generating insert sql")
)

func errMisMatchedRowLength(expectedL, actualL int) error {
	return errors.New("rows with different value length expected %d got %d", expectedL, actualL)
}

func errUpsertWithWhereNotSupported(dialect string) error {
	return NewUnsupportedError("dialect does not support upsert with where clause [dialect=%s]", dialect)
}

func errOnConflictNotSupported(dialect string) error {
	return NewUnsupportedError("dialect does not support upsert [dialect=%s]", dialect)
}

func errExcludedRequiresInsertCols(dialect string) error {
	return NewUnsupportedError("dialect requires Cols to use Excluded with INSERT ... SELECT [dialect=%s]", dialect)
}

func NewInsertSQLGenerator(dialect, tagName string, do *SQLDialectOptions) InsertSQLGenerator {
	return &insertSQLGenerator{NewCommonSQLGenerator(dialect, tagName, do)}
}

func (isg *insertSQLGenerator) Generate(
	b sb.SQLBuilder,
	clauses exp.InsertClauses,
) {
	if !clauses.HasInto() {
		b.SetError(ErrNoSourceForInsert)
		return
	}
	for _, f := range isg.DialectOptions().InsertSQLOrder {
		if b.Error() != nil {
			return
		}
		switch f {
		case CommonTableSQLFragment:
			isg.ExpressionSQLGenerator().Generate(b, clauses.CommonTables())
		case InsertBeingSQLFragment:
			isg.InsertBeginSQL(b, clauses.OnConflict())
		case IntoSQLFragment:
			b.WriteRunes(isg.DialectOptions().SpaceRune)
			isg.ExpressionSQLGenerator().Generate(b, clauses.Into())
		case InsertSQLFragment:
			isg.InsertSQL(b, clauses)
		case ReturningSQLFragment:
			isg.ReturningSQL(b, clauses.Returning())
		default:
			b.SetError(ErrNotSupportedFragment("INSERT", f))
		}
	}
}

// Adds the correct fragment to being an INSERT statement
func (isg *insertSQLGenerator) InsertBeginSQL(b sb.SQLBuilder, o exp.ConflictExpression) {
	_, isNotDoNothingConflictExpression := o.(exp.ConflictUpdateExpression)
	// A conflict target requires a real ON CONFLICT clause: INSERT IGNORE / INSERT OR IGNORE would swallow
	// violations of every constraint, not just the targeted one.
	hasTarget := false
	if dn, ok := o.(exp.ConflictNothingExpression); ok {
		hasTarget = dn.Cols() != nil && len(dn.Cols().Columns()) > 0
	}
	if isg.DialectOptions().SupportsInsertIgnoreSyntax && o != nil && !isNotDoNothingConflictExpression && !hasTarget {
		b.Write(isg.DialectOptions().InsertIgnoreClause)
	} else {
		b.Write(isg.DialectOptions().InsertClause)
	}
}

// Adds the columns list to an insert statement
func (isg *insertSQLGenerator) InsertSQL(b sb.SQLBuilder, ic exp.InsertClauses) {
	rowAlias := isg.excludedRowAlias(b.IsPrepared(), ic)
	switch {
	case ic.HasRows():
		ie, err := exp.NewInsertExpression(isg.TagName(), ic.Rows()...)
		if err != nil {
			b.SetError(err)
			return
		}
		if ie.IsInsertFrom() && rowAlias != nil {
			b.SetError(errExcludedRequiresInsertCols(isg.Dialect()))
			return
		}
		isg.InsertExpressionSQL(b, ie)
	case ic.HasCols() && ic.HasVals():
		isg.insertColumnsSQL(b, ic.Cols())
		isg.insertValuesSQL(b, ic.Vals())
	case ic.HasCols() && ic.HasFrom():
		isg.insertColumnsSQL(b, ic.Cols())
		if rowAlias != nil {
			isg.insertFromRowAliasSQL(b, ic.From(), ic.Cols(), rowAlias)
			isg.onConflictSQL(b, ic.OnConflict(), rowAlias)
			return
		}
		isg.insertFromSQL(b, ic.From())
	case ic.HasFrom():
		if rowAlias != nil {
			b.SetError(errExcludedRequiresInsertCols(isg.Dialect()))
			return
		}
		isg.insertFromSQL(b, ic.From())
	default:
		isg.defaultValuesSQL(b)
	}
	if ic.HasAlias() {
		rowAlias = ic.Alias()
	}
	if rowAlias != nil {
		b.Write(isg.DialectOptions().AsFragment)
		isg.ExpressionSQLGenerator().Generate(b, rowAlias)
	}
	isg.onConflictSQL(b, ic.OnConflict(), rowAlias)
}

func (isg *insertSQLGenerator) excludedRowAlias(isPrepared bool, ic exp.InsertClauses) exp.IdentifierExpression {
	if isg.DialectOptions().ExcludedStyle != ExcludedRowAliasStyle {
		return nil
	}
	cu, ok := ic.OnConflict().(exp.ConflictUpdateExpression)
	if !ok {
		return nil
	}
	alias := exp.NewIdentifierExpression("", isg.DialectOptions().ExcludedRowAlias, nil)
	if ic.HasAlias() {
		alias = ic.Alias()
	}
	probe := &conflictUpdateBuilder{SQLBuilder: sb.NewSQLBuilder(isPrepared), rowAlias: alias}
	defer sb.ReleaseSQLBuilder(probe.SQLBuilder)
	isg.conflictUpdateSQL(probe, cu)
	if !probe.usesExcluded {
		return nil
	}
	return alias
}

func (isg *insertSQLGenerator) insertFromRowAliasSQL(
	b sb.SQLBuilder,
	from exp.AppendableExpression,
	cols exp.ColumnListExpression,
	rowAlias exp.IdentifierExpression,
) {
	b.Write(selectAllFromFragment)
	from.AppendSQL(b)
	b.WriteRunes(isg.DialectOptions().RightParenRune)
	b.Write(isg.DialectOptions().AsFragment)
	isg.ExpressionSQLGenerator().Generate(b, rowAlias)
	isg.insertColumnsSQL(b, unqualifiedColumns(cols))
}

func unqualifiedColumns(cols exp.ColumnListExpression) exp.ColumnListExpression {
	names := make([]interface{}, 0, len(cols.Columns()))
	for _, col := range cols.Columns() {
		if ident, ok := col.(exp.IdentifierExpression); ok {
			col = exp.NewIdentifierExpression("", "", ident.GetCol())
		}
		names = append(names, col)
	}
	return exp.NewColumnListExpression(nil, "", names...)
}

func (isg *insertSQLGenerator) InsertExpressionSQL(b sb.SQLBuilder, ie exp.InsertExpression) {
	switch {
	case ie.IsInsertFrom():
		isg.insertFromSQL(b, ie.From())
	case ie.IsEmpty():
		isg.defaultValuesSQL(b)
	default:
		isg.insertColumnsSQL(b, ie.Cols())
		isg.insertValuesSQL(b, ie.Vals())
	}
}

// Adds the DefaultValuesFragment to an SQL statement
func (isg *insertSQLGenerator) defaultValuesSQL(b sb.SQLBuilder) {
	b.Write(isg.DialectOptions().DefaultValuesFragment)
}

func (isg *insertSQLGenerator) insertFromSQL(b sb.SQLBuilder, ae exp.AppendableExpression) {
	b.WriteRunes(isg.DialectOptions().SpaceRune)
	ae.AppendSQL(b)
}

// Adds the columns list to an insert statement
func (isg *insertSQLGenerator) insertColumnsSQL(b sb.SQLBuilder, cols exp.ColumnListExpression) {
	b.WriteRunes(isg.DialectOptions().SpaceRune, isg.DialectOptions().LeftParenRune)
	isg.ExpressionSQLGenerator().Generate(b, cols)
	b.WriteRunes(isg.DialectOptions().RightParenRune)
}

// Adds the values clause to an SQL statement
func (isg *insertSQLGenerator) insertValuesSQL(b sb.SQLBuilder, values [][]interface{}) {
	b.Write(isg.DialectOptions().ValuesFragment)
	rowLen := len(values[0])
	valueLen := len(values)
	for i, row := range values {
		if len(row) != rowLen {
			b.SetError(errMisMatchedRowLength(rowLen, len(row)))
			return
		}
		isg.ExpressionSQLGenerator().Generate(b, row)
		if i < valueLen-1 {
			b.WriteRunes(isg.DialectOptions().CommaRune, isg.DialectOptions().SpaceRune)
		}
	}
}

// Adds the DefaultValuesFragment to an SQL statement
func (isg *insertSQLGenerator) onConflictSQL(b sb.SQLBuilder, o exp.ConflictExpression, rowAlias exp.IdentifierExpression) {
	if o == nil {
		return
	}
	if !isg.DialectOptions().SupportsOnConflict {
		b.SetError(errOnConflictNotSupported(isg.Dialect()))
		return
	}
	b.Write(isg.DialectOptions().ConflictFragment)
	switch t := o.(type) {
	case exp.ConflictUpdateExpression:
		target := t.TargetColumn()
		if isg.DialectOptions().SupportsConflictTarget && target != "" {
			wrapParens := !strings.HasPrefix(strings.ToLower(target), "on constraint")

			b.WriteRunes(isg.DialectOptions().SpaceRune)
			if wrapParens {
				b.WriteRunes(isg.DialectOptions().LeftParenRune).
					WriteStrings(target).
					WriteRunes(isg.DialectOptions().RightParenRune)
			} else {
				b.Write([]byte(target))
			}
		}
		isg.onConflictDoUpdateSQL(b, t, rowAlias)
	case exp.ConflictNothingExpression:
		isg.ConflictTargetSQL(b, t.Cols())
		if b.Error() != nil {
			return
		}
		b.Write(isg.DialectOptions().ConflictDoNothingFragment)
	default:
		b.Write(isg.DialectOptions().ConflictDoNothingFragment)
	}
}

func (isg *insertSQLGenerator) onConflictDoUpdateSQL(
	b sb.SQLBuilder,
	o exp.ConflictUpdateExpression,
	rowAlias exp.IdentifierExpression,
) {
	b.Write(isg.DialectOptions().ConflictDoUpdateFragment)
	isg.conflictUpdateSQL(&conflictUpdateBuilder{SQLBuilder: b, rowAlias: rowAlias}, o)
}

func (isg *insertSQLGenerator) conflictUpdateSQL(b *conflictUpdateBuilder, o exp.ConflictUpdateExpression) {
	update := o.Update()
	if update == nil {
		b.SetError(ErrConflictUpdateValuesRequired)
		return
	}
	ue, err := exp.NewUpdateExpressions(isg.TagName(), update)
	if err != nil {
		b.SetError(err)
		return
	}
	for _, u := range ue {
		if e, ok := u.Val().(exp.ExcludedExpression); ok && e.Col() == "" {
			b.SetError(errExcludedWithoutColumn)
			return
		}
	}
	isg.UpdateExpressionSQL(b, ue...)
	if b.Error() == nil && o.WhereClause() != nil {
		if !isg.DialectOptions().SupportsConflictUpdateWhere {
			b.SetError(errUpsertWithWhereNotSupported(isg.Dialect()))
			return
		}
		isg.WhereSQL(b, o.WhereClause())
	}
}
