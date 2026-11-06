package sqlgen_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

func BenchmarkInsertSQLGenerator_Upsert(b *testing.B) {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b", "c")).
		SetVals([][]interface{}{{1, "b", 3}})
	plain := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x", "c": 4}))
	excluded := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
		"b": exp.NewExcludedExpression("b"),
		"c": exp.NewExcludedExpression("c"),
	}))
	rowAliasOpts := sqlgen.DefaultDialectOptions()
	rowAliasOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	for _, bc := range []struct {
		name   string
		opts   *sqlgen.SQLDialectOptions
		clause exp.InsertClauses
	}{
		{"table/plain", sqlgen.DefaultDialectOptions(), plain},
		{"table/excluded", sqlgen.DefaultDialectOptions(), excluded},
		{"rowalias/plain", rowAliasOpts, plain},
		{"rowalias/excluded", rowAliasOpts, excluded},
	} {
		isg := sqlgen.NewInsertSQLGenerator("bench", "db", bc.opts)
		for _, prepared := range []bool{false, true} {
			name := bc.name + "/interpolated"
			if prepared {
				name = bc.name + "/prepared"
			}
			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					builder := sb.NewSQLBuilder(prepared)
					isg.Generate(builder, bc.clause)
					if _, _, err := builder.ToSQL(); err != nil {
						b.Fatal(err)
					}
					sb.ReleaseSQLBuilder(builder)
				}
			})
		}
	}
}
