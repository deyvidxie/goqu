package sqlgen_test

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
)

func TestIsUnsupported(t *testing.T) {
	err := sqlgen.NewUnsupportedError("dialect does not support X [dialect=%s]", "test")
	assert.EqualError(t, err, "goqu: dialect does not support X [dialect=test]")
	assert.True(t, sqlgen.IsUnsupported(err))
	assert.True(t, sqlgen.IsUnsupported(fmt.Errorf("building query: %w", err)))
	assert.False(t, sqlgen.IsUnsupported(sqlgen.ErrInvalidCollation("x")))
	assert.False(t, sqlgen.IsUnsupported(nil))
}

func TestUnsupportedFeaturesReturnUnsupportedError(t *testing.T) {
	type sqlBuilder interface {
		ToSQL() (string, []interface{}, error)
	}
	for _, c := range []struct {
		name    string
		disable func(*sqlgen.SQLDialectOptions)
		build   func(goqu.DialectWrapper) sqlBuilder
	}{
		{"CTE", func(o *sqlgen.SQLDialectOptions) { o.SupportsWithCTE = false }, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").With("c", d.From("x"))
		}},
		{"recursive CTE", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsWithCTERecursive = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").WithRecursive("c", d.From("x"))
		}},
		{"RETURNING", func(o *sqlgen.SQLDialectOptions) { o.SupportsReturn = false }, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).Returning("a")
		}},
		{"conflict target", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictTarget = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).
				OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
		}},
		{"upsert WHERE", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictUpdateWhere = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).
				OnConflict(goqu.DoUpdate("a", goqu.Record{"a": 2}).Where(goqu.C("a").Eq(1)))
		}},
		{"UPDATE multiple tables", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsMultipleUpdateTables = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Update("t").Set(goqu.Record{"a": 1}).From("u")
		}},
		{"UPDATE ORDER BY", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedOrderByOnUpdate = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Update("t").Set(goqu.Record{"a": 1}).Order(goqu.C("a").Asc())
		}},
		{"UPDATE LIMIT", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedLimitOnUpdate = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Update("t").Set(goqu.Record{"a": 1}).Limit(1)
		}},
		{"DELETE ORDER BY", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedOrderByOnDelete = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Delete("t").Order(goqu.C("a").Asc())
		}},
		{"DELETE LIMIT", func(o *sqlgen.SQLDialectOptions) {
			o.ErrorOnUnsupportedLimitOnDelete = true
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Delete("t").Limit(1)
		}},
		{"DISTINCT ON", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsDistinctOn = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Distinct("a")
		}},
		{"WINDOW", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsWindowFunction = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Window(goqu.W("w"))
		}},
		{"QUALIFY", func(o *sqlgen.SQLDialectOptions) { o.SupportsQualify = false }, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Qualify(goqu.C("a").Eq(1))
		}},
		{"lock strength", func(o *sqlgen.SQLDialectOptions) {
			o.UnsupportedLockStrengths = []exp.LockStrength{exp.ForShare}
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").ForShare(goqu.Wait)
		}},
		{"LATERAL", func(o *sqlgen.SQLDialectOptions) { o.SupportsLateral = false }, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Join(goqu.Lateral(d.From("x")), goqu.On(goqu.V(true)))
		}},
		{"boolean data type", func(o *sqlgen.SQLDialectOptions) {
			o.BooleanDataTypeSupported = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Is(true))
		}},
		{"boolean operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.BooleanOperatorLookup, exp.LikeOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Like("x%"))
		}},
		{"bitwise operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.BitwiseOperatorLookup, exp.BitwiseInversionOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").BitwiseInversion())
		}},
		{"arithmetic operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.ArithmeticOperatorLookup, exp.ArithmeticAddOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Select(goqu.C("a").Add(1))
		}},
		{"range operator", func(o *sqlgen.SQLDialectOptions) {
			delete(o.RangeOperatorLookup, exp.BetweenOp)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Between(exp.NewRangeVal(1, 2)))
		}},
		{"join type", func(o *sqlgen.SQLDialectOptions) {
			delete(o.JoinTypeLookup, exp.FullOuterJoinType)
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").FullOuterJoin(goqu.T("u"), goqu.On(goqu.V(true)))
		}},
		{"STRAIGHT_JOIN USING", func(o *sqlgen.SQLDialectOptions) {
			o.JoinTypeLookup[exp.StraightJoinType] = []byte(" STRAIGHT_JOIN ")
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").StraightJoin(goqu.T("u"), goqu.Using("a"))
		}},
	} {
		opts := sqlgen.DefaultDialectOptions()
		c.disable(opts)
		dialect := "unsupported-" + c.name
		goqu.RegisterDialect(dialect, "db", opts)
		_, _, err := c.build(goqu.Dialect(dialect)).ToSQL()
		assert.Error(t, err, c.name)
		assert.True(t, sqlgen.IsUnsupported(err), "%s: %v", c.name, err)
	}
}
