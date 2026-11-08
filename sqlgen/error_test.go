package sqlgen_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
)

//nolint:funlen
func TestUnsupportedFeaturesReturnUnsupportedError(t *testing.T) {
	type sqlBuilder interface {
		ToSQL() (string, []interface{}, error)
	}
	for _, c := range []struct {
		name      string
		want      string
		configure func(*sqlgen.SQLDialectOptions)
		build     func(goqu.DialectWrapper) sqlBuilder
	}{
		{"CTE", "CTE WITH clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsWithCTE = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").With("c", d.From("x"))
		}},
		{"recursive CTE", "CTE WITH RECURSIVE clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsWithCTERecursive = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").WithRecursive("c", d.From("x"))
		}},
		{"RETURNING", "RETURNING clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsReturn = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).Returning("a")
		}},
		{"conflict target", "conflict target clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictTarget = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).
				OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
		}},
		{"DoUpdate target", "conflict target clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictTarget = false
			o.ErrorOnUnsupportedConflictTarget = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).OnConflict(goqu.DoUpdate("a", goqu.Record{"a": 2}))
		}},
		{"upsert", "does not support upsert [dialect=", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsOnConflict = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).OnConflict(goqu.DoNothing())
		}},
		{"upsert WHERE", "upsert with where clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictUpdateWhere = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).
				OnConflict(goqu.DoUpdate("a", goqu.Record{"a": 2}).Where(goqu.C("a").Eq(1)))
		}},
		{"unknown ExcludedStyle", "unknown ExcludedStyle", func(o *sqlgen.SQLDialectOptions) {
			o.ExcludedStyle = sqlgen.ExcludedRowAliasStyle + 1
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).OnConflict(goqu.DoUpdateExcluded("a", "b"))
		}},
		{"Excluded with INSERT SELECT without Cols", "requires Cols(...).FromQuery(...) to use Excluded", func(o *sqlgen.SQLDialectOptions) {
			o.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").FromQuery(d.From("s")).OnConflict(goqu.DoUpdateExcluded("a", "b"))
		}},
		{"UPDATE multiple tables", "multiple tables in UPDATE", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsMultipleUpdateTables = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Update("t").Set(goqu.Record{"a": 1}).From("u")
		}},
		{"UPDATE ORDER BY", "ORDER BY on UPDATE", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedOrderByOnUpdate = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Update("t").Set(goqu.Record{"a": 1}).Order(goqu.C("a").Asc())
		}},
		{"UPDATE LIMIT", "LIMIT on UPDATE", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedLimitOnUpdate = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Update("t").Set(goqu.Record{"a": 1}).Limit(1)
		}},
		{"DELETE ORDER BY", "ORDER BY on DELETE", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedOrderByOnDelete = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Delete("t").Order(goqu.C("a").Asc())
		}},
		{"DELETE LIMIT", "LIMIT on DELETE", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedLimitOnDelete = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Delete("t").Limit(1)
		}},
		{"DISTINCT ON", "DISTINCT ON clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsDistinctOn = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Distinct("a")
		}},
		{"WINDOW", "WINDOW clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsWindowFunction = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Window(goqu.W("w"))
		}},
		{"QUALIFY", "QUALIFY clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsQualify = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Qualify(goqu.C("a").Eq(1))
		}},
		{"lock strength", "FOR SHARE", func(o *sqlgen.SQLDialectOptions) {
			o.UnsupportedLockStrengths = []exp.LockStrength{exp.ForShare}
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").ForShare(goqu.Wait)
		}},
		{"lock wait option", "SKIP LOCKED", func(o *sqlgen.SQLDialectOptions) {
			o.UnsupportedLockWaitOptions = []exp.WaitOption{exp.SkipLocked}
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").ForUpdate(goqu.SkipLocked)
		}},
		{"lock OF", "OF in a locking clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsLockOf = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").ForUpdate(goqu.Wait, goqu.T("t"))
		}},
		{"JSON functions", "JSONValue", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsMySQLJSONFunctions = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.JSONValue("doc", "$.a").Eq(1))
		}},
		{"Interval", "Interval", func(o *sqlgen.SQLDialectOptions) {
			o.TimeStyle = sqlgen.SQLServerTimeStyle
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Lt(goqu.Now().Sub(goqu.Interval(1, "day"))))
		}},
		{"fsp", "fsp 6, want one of 0, 3", func(o *sqlgen.SQLDialectOptions) {
			o.TimeStyle = sqlgen.SQLiteTimeStyle
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Select(goqu.Now(6))
		}},
		{"LATERAL", "lateral expressions", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsLateral = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Join(goqu.Lateral(d.From("x")), goqu.On(goqu.V(true)))
		}},
		{"boolean data type", "dialect does not support the boolean data type", func(o *sqlgen.SQLDialectOptions) {
			o.BooleanDataTypeSupported = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Is(true))
		}},
		{"boolean operator", "dialect does not support boolean operator \"like\"", func(o *sqlgen.SQLDialectOptions) {
			delete(o.BooleanOperatorLookup, exp.LikeOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Like("x%"))
		}},
		{"bitwise operator", "bitwise operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.BitwiseOperatorLookup, exp.BitwiseInversionOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").BitwiseInversion())
		}},
		{"arithmetic operator", "arithmetic operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.ArithmeticOperatorLookup, exp.ArithmeticAddOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Select(goqu.C("a").Add(1))
		}},
		{"range operator", "range operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.RangeOperatorLookup, exp.BetweenOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Between(exp.NewRangeVal(1, 2)))
		}},
		{"join type", "does not support FullOuterJoinType", func(o *sqlgen.SQLDialectOptions) {
			delete(o.JoinTypeLookup, exp.FullOuterJoinType)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").FullOuterJoin(goqu.T("u"), goqu.On(goqu.V(true)))
		}},
		{"STRAIGHT_JOIN USING", "USING is not supported", func(o *sqlgen.SQLDialectOptions) {
			o.JoinTypeLookup[exp.StraightJoinType] = []byte(" STRAIGHT_JOIN ")
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").StraightJoin(goqu.T("u"), goqu.Using("a"))
		}},
	} {
		opts := sqlgen.DefaultDialectOptions()
		c.configure(opts)
		dialect := "unsupported-" + c.name
		goqu.RegisterDialect(dialect, "db", opts)
		_, _, err := c.build(goqu.Dialect(dialect)).ToSQL()
		assert.ErrorContains(t, err, c.want, c.name)
		assert.True(t, goqu.IsUnsupported(err), "%s: %v", c.name, err)
	}
}

type unknownErrorTestExpression struct{}

func (unknownErrorTestExpression) Expression() exp.Expression { return unknownErrorTestExpression{} }
func (unknownErrorTestExpression) Clone() exp.Expression      { return unknownErrorTestExpression{} }

func TestGeneratorErrorsAreTyped(t *testing.T) {
	opts := sqlgen.DefaultDialectOptions()
	toSQL := func(generate func(sb.SQLBuilder)) error {
		b := sb.NewSQLBuilder(false)
		generate(b)
		_, _, err := b.ToSQL()
		return err
	}
	expression := func(e any) error {
		return toSQL(func(b sb.SQLBuilder) { sqlgen.NewExpressionSQLGenerator("test", opts).Generate(b, e) })
	}
	mismatched := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "t", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]any{{1}, {1, 2}})
	badFragment := sqlgen.DefaultDialectOptions()
	badFragment.SelectSQLOrder = []sqlgen.SQLFragmentType{sqlgen.InsertBeingSQLFragment}

	invalid := []struct {
		name string
		want string
		err  error
	}{
		{"unsupported expression type", "unsupported expression type", expression(unknownErrorTestExpression{})},
		{
			"unsupported identifier column", "unsupported column type",
			expression(exp.NewIdentifierExpression("", "", false)),
		},
		{"unencodable value", "unable to encode value {}", expression(struct{}{})},
		{"mismatched row length", "rows with different value length, expected 1 got 2", toSQL(func(b sb.SQLBuilder) {
			sqlgen.NewInsertSQLGenerator("test", "db", opts).Generate(b, mismatched)
		})},
		{"truncate without source", "no source found when generating truncate sql", toSQL(func(b sb.SQLBuilder) {
			sqlgen.NewTruncateSQLGenerator("test", "db", opts).Generate(b, exp.NewTruncateClauses())
		})},
	}
	for _, c := range invalid {
		assert.ErrorContains(t, c.err, c.want, c.name)
		assert.True(t, goqu.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}

	err := toSQL(func(b sb.SQLBuilder) {
		sqlgen.NewSelectSQLGenerator("test", "db", badFragment).Generate(b, exp.NewSelectClauses())
	})
	assert.EqualError(t, err, "goqu: unsupported SELECT SQL fragment InsertBeingSQLFragment [dialect=test]")
	assert.True(t, goqu.IsUnsupported(err))
}
