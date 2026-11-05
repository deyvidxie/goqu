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
		want    string
		disable func(*sqlgen.SQLDialectOptions)
		build   func(goqu.DialectWrapper) sqlBuilder
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
		{"conflict target", "Conflict Target clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictTarget = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).
				OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
		}},
		{"upsert WHERE", "upsert with where clause", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsConflictUpdateWhere = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.Insert("t").Rows(goqu.Record{"a": 1}).
				OnConflict(goqu.DoUpdate("a", goqu.Record{"a": 2}).Where(goqu.C("a").Eq(1)))
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
		{"LATERAL", "lateral expressions", func(o *sqlgen.SQLDialectOptions) {
			o.SupportsLateral = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Join(goqu.Lateral(d.From("x")), goqu.On(goqu.V(true)))
		}},
		{"boolean data type", "boolean data type is not supported", func(o *sqlgen.SQLDialectOptions) {
			o.BooleanDataTypeSupported = false
		}, func(d goqu.DialectWrapper) sqlBuilder {
			return d.From("t").Where(goqu.C("a").Is(true))
		}},
		{"boolean operator", "boolean operator 'like' not supported", func(o *sqlgen.SQLDialectOptions) {
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
		c.disable(opts)
		dialect := "unsupported-" + c.name
		goqu.RegisterDialect(dialect, "db", opts)
		_, _, err := c.build(goqu.Dialect(dialect)).ToSQL()
		assert.ErrorContains(t, err, c.want, c.name)
		assert.True(t, sqlgen.IsUnsupported(err), "%s: %v", c.name, err)
	}
}
