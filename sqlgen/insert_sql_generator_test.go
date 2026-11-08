package sqlgen_test

import (
	"database/sql/driver"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type (
	insertTestCase struct {
		clause     exp.InsertClauses
		sql        string
		isPrepared bool
		args       []interface{}
		err        string
	}
	insertSQLGeneratorSuite struct {
		baseSQLGeneratorSuite
	}
)

func (igs *insertSQLGeneratorSuite) assertCases(isg sqlgen.InsertSQLGenerator, testCases ...insertTestCase) {
	for _, tc := range testCases {
		b := sb.NewSQLBuilder(tc.isPrepared)
		isg.Generate(b, tc.clause)
		switch {
		case tc.err != "":
			igs.assertErrorSQL(b, tc.err)
		case tc.isPrepared:
			igs.assertPreparedSQL(b, tc.sql, tc.args)
		default:
			igs.assertNotPreparedSQL(b, tc.sql)
		}
	}
}

func (igs *insertSQLGeneratorSuite) TestDialect() {
	opts := sqlgen.DefaultDialectOptions()
	d := sqlgen.NewInsertSQLGenerator("test", "db", opts)
	igs.Equal("test", d.Dialect())

	opts2 := sqlgen.DefaultDialectOptions()
	d2 := sqlgen.NewInsertSQLGenerator("test2", "db", opts2)
	igs.Equal("test2", d2.Dialect())
}

func (igs *insertSQLGeneratorSuite) TestGenerate_UnsupportedFragment() {
	opts := sqlgen.DefaultDialectOptions()
	opts.InsertSQLOrder = []sqlgen.SQLFragmentType{sqlgen.UpdateBeginSQLFragment}
	d := sqlgen.NewInsertSQLGenerator("test", "db", opts)

	b := sb.NewSQLBuilder(true)
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", ""))
	d.Generate(b, ic)
	igs.assertErrorSQL(b, `goqu: dialect does not support INSERT SQL fragment UpdateBeginSQLFragment [dialect=test]`)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_empty() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", ""))

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" DEFAULT VALUES`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" DEFAULT VALUES`, isPrepared: true},
	)

	opts2 := sqlgen.DefaultDialectOptions()
	opts2.DefaultValuesFragment = []byte(" default values")

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts2),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" default values`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" default values`, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_nilValues() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]interface{}{
			{nil},
		})

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a") VALUES (NULL)`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a") VALUES (?)`, isPrepared: true, args: []interface{}{nil}},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_colsAndVals() {
	opts := sqlgen.DefaultDialectOptions()
	opts.LeftParenRune = '{'
	opts.RightParenRune = '}'
	opts.ValuesFragment = []byte(" values ")
	opts.LeftParenRune = '{'
	opts.RightParenRune = '}'
	opts.CommaRune = ';'
	opts.PlaceHolderFragment = []byte("#")

	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{
			{"a1", "b1"},
			{"a2", "b2"},
			{"a3", "b3"},
		})

	bic := ic.SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{
			{"a1"},
			{"a2", "b2"},
			{"a3", "b3"},
		})

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" {"a"; "b"} values {'a1'; 'b1'}; {'a2'; 'b2'}; {'a3'; 'b3'}`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" {"a"; "b"} values {#; #}; {#; #}; {#; #}`, isPrepared: true, args: []interface{}{
			"a1", "b1", "a2", "b2", "a3", "b3",
		}},

		insertTestCase{clause: bic, err: `goqu: rows with different value length, expected 1 got 2`},
		insertTestCase{clause: bic, err: `goqu: rows with different value length, expected 1 got 2`, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withNoInto() {
	opts := sqlgen.DefaultDialectOptions()
	opts.LeftParenRune = '{'
	opts.RightParenRune = '}'
	opts.ValuesFragment = []byte(" values ")
	opts.LeftParenRune = '{'
	opts.RightParenRune = '}'
	opts.CommaRune = ';'
	opts.PlaceHolderFragment = []byte("#")

	ic := exp.NewInsertClauses().
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{
			{"a1", "b1"},
			{"a2", "b2"},
			{"a3", "b3"},
		})
	expectedErr := "goqu: no source found when generating insert sql"
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: ic, err: expectedErr},
		insertTestCase{clause: ic, err: expectedErr, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withRows() {
	opts := sqlgen.DefaultDialectOptions()
	opts.LeftParenRune = '{'
	opts.RightParenRune = '}'
	opts.ValuesFragment = []byte(" values ")
	opts.LeftParenRune = '{'
	opts.RightParenRune = '}'
	opts.CommaRune = ';'
	opts.PlaceHolderFragment = []byte("#")

	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetRows([]interface{}{
			exp.Record{"a": "a1", "b": "b1"},
			exp.Record{"a": "a2", "b": "b2"},
			exp.Record{"a": "a3", "b": "b3"},
		})

	bic := ic.SetRows([]interface{}{
		exp.Record{"a": "a1"},
		exp.Record{"a": "a2", "b": "b2"},
		exp.Record{"a": "a3", "b": "b3"},
	})

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" {"a"; "b"} values {'a1'; 'b1'}; {'a2'; 'b2'}; {'a3'; 'b3'}`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" {"a"; "b"} values {#; #}; {#; #}; {#; #}`, isPrepared: true, args: []interface{}{
			"a1", "b1", "a2", "b2", "a3", "b3",
		}},

		insertTestCase{clause: bic, err: `goqu: rows with different value length, expected 1 got 2`},
		insertTestCase{clause: bic, err: `goqu: rows with different value length, expected 1 got 2`, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withEmptyRows() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetRows([]interface{}{exp.Record{}})

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" DEFAULT VALUES`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" DEFAULT VALUES`, isPrepared: true},
	)

	opts2 := sqlgen.DefaultDialectOptions()
	opts2.DefaultValuesFragment = []byte(" default values")

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts2),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" default values`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" default values`, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withRowsAppendableExpression() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetRows([]interface{}{newTestAppendableExpression(`select * from "other"`, emptyArgs, nil, nil)})

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" select * from "other"`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" select * from "other"`, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withFrom() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetFrom(newTestAppendableExpression(`select c, d from test where a = 'b'`, nil, nil, nil))

	icCols := ic.SetCols(exp.NewColumnListExpression(nil, "db", "a", "b"))
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" select c, d from test where a = 'b'`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" select c, d from test where a = 'b'`, isPrepared: true},

		insertTestCase{clause: icCols, sql: `INSERT INTO "test" ("a", "b") select c, d from test where a = 'b'`},
		insertTestCase{clause: icCols, sql: `INSERT INTO "test" ("a", "b") select c, d from test where a = 'b'`, isPrepared: true},
		insertTestCase{
			clause: icCols.SetAlias(exp.NewIdentifierExpression("", "new", nil)).
				SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})),
			sql: `INSERT INTO "test" ("a", "b") select c, d from test where a = 'b' AS "new" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"=EXCLUDED."b"`,
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflict() {
	opts := sqlgen.DefaultDialectOptions()
	// make sure the fragments are used
	opts.ConflictFragment = []byte(" on conflict")
	opts.ConflictDoNothingFragment = []byte(" do nothing")
	opts.ConflictDoUpdateFragment = []byte(" do update set ")

	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]interface{}{
			{"a1"},
		})
	icDn := ic.SetOnConflict(exp.NewDoNothingConflictExpression())
	icDu := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("test", exp.Record{"a": "b"}))
	icAsDu := ic.SetAlias(exp.NewIdentifierExpression("", "new", "")).SetOnConflict(
		exp.NewDoUpdateConflictExpression("test", exp.Record{"a": exp.NewIdentifierExpression("", "new", "a")}),
	)
	icDoc := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("on constraint test", exp.Record{"a": "b"}))
	icDuw := ic.SetOnConflict(
		exp.NewDoUpdateConflictExpression("test", exp.Record{"a": "b"}).Where(exp.Ex{"foo": true}),
	)

	icDuNil := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("test", nil))
	icDuBad := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("test", true))

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDn, sql: `INSERT INTO "test" ("a") VALUES ('a1') on conflict do nothing`},
		insertTestCase{
			clause:     icDn,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict do nothing`,
			isPrepared: true,
			args:       []interface{}{"a1"},
		},

		insertTestCase{clause: icDu, sql: `INSERT INTO "test" ("a") VALUES ('a1') on conflict (test) do update set "a"='b'`},
		insertTestCase{
			clause:     icDu,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict (test) do update set "a"=?`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},

		insertTestCase{clause: icAsDu, sql: `INSERT INTO "test" ("a") VALUES ('a1') AS "new" on conflict (test) do update set "a"="new"."a"`},
		insertTestCase{
			clause:     icAsDu,
			sql:        `INSERT INTO "test" ("a") VALUES (?) AS "new" on conflict (test) do update set "a"="new"."a"`,
			isPrepared: true,
			args:       []interface{}{"a1"},
		},

		insertTestCase{clause: icDoc, sql: `INSERT INTO "test" ("a") VALUES ('a1') on conflict on constraint test do update set "a"='b'`},
		insertTestCase{
			clause:     icDoc,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict on constraint test do update set "a"=?`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},

		insertTestCase{
			clause: icDuw,
			sql:    `INSERT INTO "test" ("a") VALUES ('a1') on conflict (test) do update set "a"='b' WHERE ("foo" IS TRUE)`,
		},
		insertTestCase{
			clause:     icDuw,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict (test) do update set "a"=? WHERE ("foo" IS TRUE)`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},

		insertTestCase{clause: icDuNil, err: "goqu: values are required for on conflict update expression"},
		insertTestCase{clause: icDuNil, err: "goqu: values are required for on conflict update expression", isPrepared: true},

		insertTestCase{clause: icDuBad, err: "goqu: unsupported update value type bool, a map, goqu.Record or struct is required"},
		insertTestCase{
			clause: icDuBad, err: "goqu: unsupported update value type bool, a map, goqu.Record or struct is required", isPrepared: true,
		},
	)
	opts.SupportsInsertIgnoreSyntax = true
	opts.InsertIgnoreClause = []byte("insert ignore into")
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDn, sql: `insert ignore into "test" ("a") VALUES ('a1') on conflict do nothing`},
		insertTestCase{
			clause:     icDn,
			sql:        `insert ignore into "test" ("a") VALUES (?) on conflict do nothing`,
			isPrepared: true,
			args:       []interface{}{"a1"},
		},

		insertTestCase{
			clause: icDu,
			sql:    `INSERT INTO "test" ("a") VALUES ('a1') on conflict (test) do update set "a"='b'`,
		},
		insertTestCase{
			clause:     icDu,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict (test) do update set "a"=?`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},

		insertTestCase{
			clause: icDoc,
			sql:    `INSERT INTO "test" ("a") VALUES ('a1') on conflict on constraint test do update set "a"='b'`,
		},
		insertTestCase{
			clause:     icDoc,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict on constraint test do update set "a"=?`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},

		insertTestCase{
			clause: icDuw,
			sql:    `INSERT INTO "test" ("a") VALUES ('a1') on conflict (test) do update set "a"='b' WHERE ("foo" IS TRUE)`,
		},
		insertTestCase{
			clause:     icDuw,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict (test) do update set "a"=? WHERE ("foo" IS TRUE)`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},

		insertTestCase{clause: icDuNil, err: "goqu: values are required for on conflict update expression"},
		insertTestCase{clause: icDuNil, err: "goqu: values are required for on conflict update expression", isPrepared: true},

		insertTestCase{clause: icDuBad, err: "goqu: unsupported update value type bool, a map, goqu.Record or struct is required"},
		insertTestCase{
			clause: icDuBad, err: "goqu: unsupported update value type bool, a map, goqu.Record or struct is required", isPrepared: true,
		},
	)

	opts.SupportsConflictUpdateWhere = false
	expectedErr := "goqu: dialect does not support upsert with where clause [dialect=test]"
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDuw, err: expectedErr},
		insertTestCase{clause: icDuw, err: expectedErr, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictDoNothingTarget() {
	opts := sqlgen.DefaultDialectOptions()
	opts.ConflictFragment = []byte(" on conflict")
	opts.ConflictDoNothingFragment = []byte(" do nothing")

	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]any{{"a1"}})
	icDnCols := ic.SetOnConflict(exp.NewDoNothingConflictExpression().SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")))
	icDnEmptyCols := ic.SetOnConflict(exp.NewDoNothingConflictExpression().SetCols(exp.NewColumnListExpression(nil, "db")))

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDnCols, sql: `INSERT INTO "test" ("a") VALUES ('a1') on conflict ("a", "b") do nothing`},
		insertTestCase{
			clause:     icDnCols,
			sql:        `INSERT INTO "test" ("a") VALUES (?) on conflict ("a", "b") do nothing`,
			isPrepared: true,
			args:       []any{"a1"},
		},
		insertTestCase{clause: icDnEmptyCols, sql: `INSERT INTO "test" ("a") VALUES ('a1') on conflict do nothing`},
	)
}

// MySQL style: plain INSERT for upserts, INSERT IGNORE only for DO NOTHING.
func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictOnDuplicateKey() {
	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsInsertIgnoreSyntax = true
	opts.InsertIgnoreClause = []byte("INSERT IGNORE INTO")
	opts.ConflictFragment = []byte("")
	opts.ConflictDoUpdateFragment = []byte(" ON DUPLICATE KEY UPDATE ")
	opts.ConflictDoNothingFragment = []byte("")
	opts.SupportsConflictTarget = false

	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]interface{}{
			{"a1"},
		})
	icDn := ic.SetOnConflict(exp.NewDoNothingConflictExpression())
	icDnCols := ic.SetOnConflict(exp.NewDoNothingConflictExpression().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
	icDnEmptyCols := ic.SetOnConflict(exp.NewDoNothingConflictExpression().SetCols(exp.NewColumnListExpression(nil, "db")))
	icDu := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("test", exp.Record{"a": "b"}))
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDnEmptyCols, sql: `INSERT IGNORE INTO "test" ("a") VALUES ('a1')`},
		insertTestCase{clause: icDn, sql: `INSERT IGNORE INTO "test" ("a") VALUES ('a1')`},
		insertTestCase{
			clause: icDnCols,
			err:    "goqu: dialect does not support conflict target clause [dialect=test]",
		},
		insertTestCase{
			clause:     icDu,
			sql:        `INSERT INTO "test" ("a") VALUES (?) ON DUPLICATE KEY UPDATE "a"=?`,
			isPrepared: true,
			args:       []interface{}{"a1", "b"},
		},
		insertTestCase{
			clause: icDu,
			sql:    `INSERT INTO "test" ("a") VALUES ('a1') ON DUPLICATE KEY UPDATE "a"='b'`,
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictConstraintTargetIgnoresCase() {
	icDoc := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]any{{"a1"}}).
		SetOnConflict(exp.NewDoUpdateConflictExpression("ON CONSTRAINT test", exp.Record{"a": "b"}))
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: icDoc, sql: `INSERT INTO "test" ("a") VALUES ('a1') ON CONFLICT ON CONSTRAINT test DO UPDATE SET "a"='b'`},
		insertTestCase{
			clause:     icDoc,
			sql:        `INSERT INTO "test" ("a") VALUES (?) ON CONFLICT ON CONSTRAINT test DO UPDATE SET "a"=?`,
			isPrepared: true,
			args:       []any{"a1", "b"},
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictTargetErrorOnUnsupported() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]any{{"a1"}})
	icDu := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("test", exp.Record{"a": "b"}))
	icDuNoTarget := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("", exp.Record{"a": "b"}))
	icDuConstraint := ic.SetOnConflict(exp.NewDoUpdateConflictExpression("ON CONSTRAINT test_a", exp.Record{"a": "b"}))
	icDn := ic.SetOnConflict(exp.NewDoNothingConflictExpression())

	opts := sqlgen.DefaultDialectOptions()
	opts.ErrorOnUnsupportedConflictTarget = true
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDu, sql: `INSERT INTO "test" ("a") VALUES ('a1') ON CONFLICT (test) DO UPDATE SET "a"='b'`},
		insertTestCase{clause: icDuNoTarget, sql: `INSERT INTO "test" ("a") VALUES ('a1') ON CONFLICT DO UPDATE SET "a"='b'`},
	)

	opts = sqlgen.DefaultDialectOptions()
	opts.SupportsInsertIgnoreSyntax = true
	opts.InsertIgnoreClause = []byte("INSERT IGNORE INTO")
	opts.ConflictFragment = []byte("")
	opts.ConflictDoUpdateFragment = []byte(" ON DUPLICATE KEY UPDATE ")
	opts.ConflictDoNothingFragment = []byte("")
	opts.SupportsConflictTarget = false
	opts.ErrorOnUnsupportedConflictTarget = true
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icDu, err: "goqu: dialect does not support conflict target clause [dialect=test]"},
		insertTestCase{clause: icDu, err: "goqu: dialect does not support conflict target clause [dialect=test]", isPrepared: true},
		insertTestCase{clause: icDuConstraint, err: "goqu: dialect does not support conflict target clause [dialect=test]"},
		insertTestCase{clause: icDuNoTarget, sql: `INSERT INTO "test" ("a") VALUES ('a1') ON DUPLICATE KEY UPDATE "a"='b'`},
		insertTestCase{clause: icDn, sql: `INSERT IGNORE INTO "test" ("a") VALUES ('a1')`},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcluded() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})
	sumB := exp.NewDoUpdateConflictExpression("a", exp.Record{
		"b": exp.NewLiteralExpression("? + ?", exp.NewIdentifierExpression("", "test", "b"), exp.NewExcludedExpression("b")),
	})
	excludedWhere := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"}).
		Where(exp.NewIdentifierExpression("", "", "b").Lt(exp.NewExcludedExpression("b")))

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"=EXCLUDED."b"`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(sumB),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"="test"."b" + EXCLUDED."b"`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(excludedWhere),
			sql: `INSERT INTO "test" ("a", "b") VALUES (?, ?) ON CONFLICT (a) DO UPDATE SET "b"=? ` +
				`WHERE ("b" < EXCLUDED."b")`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1", "x"},
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression(1)})),
			err:    "goqu: Excluded requires a column name",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", []exp.UpdateExpression{
				exp.NewIdentifierExpression("", "", "").Set(exp.NewExcludedExpression(1)),
			})),
			err: "goqu: Excluded requires a column name",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"b": exp.NewExcludedExpression(1).Add(1),
			})),
			err: "goqu: Excluded requires a column name",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"}).
				Where(exp.NewExcludedExpression(1).Gt(1))),
			err: "goqu: Excluded requires a column name",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"b": exp.NewExcludedExpression("b").Add(1),
			}).Where(exp.NewExcludedExpression("b").Gt(exp.NewIdentifierExpression("", "test", "b")))),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"=(EXCLUDED."b" + 1) ` +
				`WHERE (EXCLUDED."b" > "test"."b")`,
		},
	)

	valuesOpts := sqlgen.DefaultDialectOptions()
	valuesOpts.ExcludedStyle = sqlgen.ExcludedValuesFuncStyle
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", valuesOpts),
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"=VALUES("b")`,
		},
		insertTestCase{
			clause:     ic.SetOnConflict(excludedB),
			sql:        `INSERT INTO "test" ("a", "b") VALUES (?, ?) ON CONFLICT (a) DO UPDATE SET "b"=VALUES("b")`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1"},
		},
	)
}

type valuerError struct{ msg string }

func (e valuerError) Error() string { return e.msg }

var errValuerFailed = valuerError{"valuer failed"}

type secondCallValuer struct {
	firstErr    error
	secondValue driver.Value
	called      bool
}

func (v *secondCallValuer) Value() (driver.Value, error) {
	if v.called {
		return v.secondValue, nil
	}
	v.called = true
	return "first", v.firstErr
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcludedRowAlias() {
	preparedOnlyValue := [2]byte{1, 2}
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	fromIc := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetFrom(newTestAppendableExpression(`select c, d from test2`, nil, nil, nil))
	fromColsIc := fromIc.SetCols(exp.NewColumnListExpression(nil, "db", "a", "b"))
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})
	sumB := exp.NewDoUpdateConflictExpression("a", exp.Record{
		"b": exp.NewLiteralExpression("? + ?", exp.NewIdentifierExpression("", "test", "b"), exp.NewExcludedExpression("b")),
	})

	aliasOpts := sqlgen.DefaultDialectOptions()
	aliasOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	aliasOpts.ExcludedRowAlias = "incoming"
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", aliasOpts),
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "incoming" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="incoming"."b"`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ("a", "b") VALUES (?, ?) AS "incoming" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="incoming"."b"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1"},
		},
		insertTestCase{
			clause: fromColsIc.SetAlias(exp.NewIdentifierExpression("", "new", nil)).SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ("a", "b") SELECT * FROM (select c, d from test2) AS "new" ("a", "b") ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="new"."b"`,
		},
		insertTestCase{
			clause: ic.SetAlias(exp.NewIdentifierExpression("", "new", nil)).SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "new" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="new"."b"`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"})),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"='x'`,
		},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1')`},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoNothingConflictExpression()),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT DO NOTHING`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": preparedOnlyValue,
				"b": exp.NewExcludedExpression("b"),
			})),
			sql: `INSERT INTO "test" ("a", "b") VALUES (?, ?) AS "incoming" ` +
				`ON CONFLICT (a) DO UPDATE SET "a"=?,"b"="incoming"."b"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1", preparedOnlyValue},
		},
		insertTestCase{
			clause: exp.NewInsertClauses().
				SetInto(exp.NewIdentifierExpression("", "test", "")).
				SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
				SetFrom(newTestAppendableExpression(`select c, d from test2 where c > ?`, []interface{}{1}, nil, nil)).
				SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
					"b": exp.NewExcludedExpression("b").Add(2),
				})),
			sql: `INSERT INTO "test" ("a", "b") SELECT * FROM (select c, d from test2 where c > ?) AS "incoming" ("a", "b") ` +
				`ON CONFLICT (a) DO UPDATE SET "b"=("incoming"."b" + ?)`,
			isPrepared: true,
			args:       []interface{}{1, int64(2)},
		},
		insertTestCase{
			clause: exp.NewInsertClauses().SetInto(exp.NewIdentifierExpression("", "test", "")).SetOnConflict(excludedB),
			sql:    `INSERT INTO "test" DEFAULT VALUES AS "incoming" ON CONFLICT (a) DO UPDATE SET "b"="incoming"."b"`,
		},
		insertTestCase{
			clause: fromColsIc.SetOnConflict(sumB),
			sql: `INSERT INTO "test" ("a", "b") SELECT * FROM (select c, d from test2) AS "incoming" ("a", "b") ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="test"."b" + "incoming"."b"`,
		},
		insertTestCase{
			clause: fromColsIc.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"})),
			sql:    `INSERT INTO "test" ("a", "b") select c, d from test2 ON CONFLICT (a) DO UPDATE SET "b"='x'`,
		},
		insertTestCase{
			clause: fromIc.SetOnConflict(excludedB),
			err:    "goqu: dialect does not support Excluded with INSERT ... SELECT without Cols(...).FromQuery(...) [dialect=test]",
		},
		insertTestCase{
			clause: exp.NewInsertClauses().
				SetInto(exp.NewIdentifierExpression("", "test", "")).
				SetRows([]interface{}{newTestAppendableExpression(`select c, d from test2`, nil, nil, nil)}).
				SetOnConflict(excludedB),
			err: "goqu: dialect does not support Excluded with INSERT ... SELECT without Cols(...).FromQuery(...) [dialect=test]",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"b": exp.NewExcludedExpression(1).Add(1),
			})),
			err: "goqu: Excluded requires a column name",
		},
		insertTestCase{
			clause: fromIc.SetCols(exp.NewColumnListExpression(nil, "db", "test.a", "b")).SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ("test"."a", "b") SELECT * FROM (select c, d from test2) AS "incoming" ("a", "b") ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="incoming"."b"`,
		},
		insertTestCase{
			clause: fromIc.SetCols(exp.NewColumnListExpression(nil, "db", exp.NewLiteralExpression("a"), "b")).
				SetOnConflict(excludedB),
			sql: `INSERT INTO "test" (a, "b") SELECT * FROM (select c, d from test2) AS "incoming" (a, "b") ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="incoming"."b"`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": &secondCallValuer{firstErr: errValuerFailed},
				"b": exp.NewExcludedExpression("b"),
			})),
			err: "goqu: driver.Valuer *sqlgen_test.secondCallValuer: valuer failed",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": &secondCallValuer{secondValue: exp.NewExcludedExpression("a")},
			})),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "a"='first'`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": &secondCallValuer{secondValue: exp.NewExcludedExpression("a")},
			})),
			sql:        `INSERT INTO "test" ("a", "b") VALUES (?, ?) ON CONFLICT (a) DO UPDATE SET "a"=?`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1", "first"},
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcludedFragments() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})

	opts := sqlgen.DefaultDialectOptions()
	opts.ExcludedTableFragment = []byte("excluded")
	opts.PeriodRune = ':'
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"=excluded:"b"`,
		},
	)

	valuesOpts := sqlgen.DefaultDialectOptions()
	valuesOpts.ExcludedStyle = sqlgen.ExcludedValuesFuncStyle
	valuesOpts.ExcludedValuesFuncFragment = []byte("values")
	valuesOpts.LeftParenRune = '['
	valuesOpts.RightParenRune = ']'
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", valuesOpts),
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			sql:    `INSERT INTO "test" ["a", "b"] VALUES ['a1', 'b1'] ON CONFLICT [a] DO UPDATE SET "b"=values["b"]`,
		},
	)

	aliasOpts := sqlgen.DefaultDialectOptions()
	aliasOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	aliasOpts.SelectClause = []byte("select")
	aliasOpts.FromFragment = []byte(" from")
	aliasOpts.StarRune = '%'
	aliasOpts.LeftParenRune = '['
	aliasOpts.RightParenRune = ']'
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", aliasOpts),
		insertTestCase{
			clause: exp.NewInsertClauses().
				SetInto(exp.NewIdentifierExpression("", "test", "")).
				SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
				SetFrom(newTestAppendableExpression(`select c, d from test2`, nil, nil, nil)).
				SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ["a", "b"] select % from [select c, d from test2] AS "excluded" ["a", "b"] ` +
				`ON CONFLICT [a] DO UPDATE SET "b"="excluded"."b"`,
		},
	)

	for _, tc := range []struct {
		style  sqlgen.ExcludedStyle
		option string
	}{
		{style: sqlgen.ExcludedTableStyle, option: "ExcludedTableFragment"},
		{style: sqlgen.ExcludedValuesFuncStyle, option: "ExcludedValuesFuncFragment"},
	} {
		emptyOpts := sqlgen.DefaultDialectOptions()
		emptyOpts.ExcludedStyle = tc.style
		emptyOpts.ExcludedTableFragment = nil
		emptyOpts.ExcludedValuesFuncFragment = nil
		b := sb.NewSQLBuilder(false)
		sqlgen.NewInsertSQLGenerator("test", "db", emptyOpts).Generate(b, ic.SetOnConflict(excludedB))
		_, _, err := b.ToSQL()
		igs.EqualError(err, "goqu: dialect has no "+tc.option+" for Excluded [dialect=test]")
		igs.True(goqu.IsUnsupported(err))
	}
}

func (igs *insertSQLGeneratorSuite) TestGenerate_insertSourcePrecedenceWithRowAlias() {
	opts := sqlgen.DefaultDialectOptions()
	opts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	from := newTestAppendableExpression(`select c, d from test2`, nil, nil, nil)
	base := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetFrom(from).
		SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")}))
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{
			clause: base.SetVals([][]interface{}{{"a1", "b1"}}),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "excluded" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="excluded"."b"`,
		},
		insertTestCase{
			clause: base.SetRows([]interface{}{exp.Record{"a": "a1", "b": "b1"}}),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "excluded" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="excluded"."b"`,
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcludedRowAliasQualifiedAndErrorOrder() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})
	opts := sqlgen.DefaultDialectOptions()
	opts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{
			clause: ic.SetAlias(exp.ParseIdentifier("t.new")).SetOnConflict(excludedB),
			err:    "goqu: Excluded requires an unqualified insert alias",
		},
		insertTestCase{
			clause: exp.NewInsertClauses().
				SetInto(exp.NewIdentifierExpression("", "test", "")).
				SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
				SetFrom(newTestAppendableExpression(`select c, d from test2`, nil, nil, nil)).
				SetAlias(exp.ParseIdentifier("t.new")).
				SetOnConflict(excludedB),
			err: "goqu: Excluded requires an unqualified insert alias",
		},
		insertTestCase{
			clause: ic.SetAlias(exp.NewIdentifierExpression("", "new", "")).SetOnConflict(excludedB),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "new" ON CONFLICT (a) DO UPDATE SET "b"="new"."b"`,
		},
		insertTestCase{
			clause: ic.SetAlias(exp.NewIdentifierExpression("s", "new", nil)).SetOnConflict(excludedB),
			err:    "goqu: Excluded requires an unqualified insert alias",
		},
		insertTestCase{
			clause: ic.SetAlias(exp.NewIdentifierExpression("s", "", "new")).SetOnConflict(excludedB),
			err:    "goqu: Excluded requires an unqualified insert alias",
		},
		insertTestCase{
			clause: ic.SetAlias(exp.NewIdentifierExpression("s", "", "new")).
				SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"})),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "s"."new" ON CONFLICT (a) DO UPDATE SET "b"='x'`,
		},
	)

	b := sb.NewSQLBuilder(false)
	sqlgen.NewInsertSQLGenerator("test", "db", opts).Generate(b, ic.SetAlias(exp.ParseIdentifier("t.new")).SetOnConflict(excludedB))
	_, _, err := b.ToSQL()
	igs.True(goqu.IsInvalidExpression(err))

	conflictValuer := &secondCallValuer{firstErr: errValuerFailed}
	b = sb.NewSQLBuilder(false)
	sqlgen.NewInsertSQLGenerator("test", "db", opts).Generate(b, ic.
		SetVals([][]interface{}{{&secondCallValuer{firstErr: valuerError{"row valuer failed"}}, "b1"}}).
		SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"a": conflictValuer})))
	_, _, err = b.ToSQL()
	igs.EqualError(err, "goqu: driver.Valuer *sqlgen_test.secondCallValuer: row valuer failed")
	igs.False(conflictValuer.called)

	numberedOpts := sqlgen.DefaultDialectOptions()
	numberedOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	numberedOpts.IncludePlaceholderNum = true
	numberedOpts.PlaceHolderFragment = []byte("$")
	for _, tc := range []struct {
		opts           *sqlgen.SQLDialectOptions
		isPrepared     bool
		err            string
		conflictCalled bool
	}{
		{opts: opts, err: "select failed"},
		{opts: opts, isPrepared: true, err: "select failed"},
		{opts: numberedOpts, err: "select failed"},
		{opts: numberedOpts, isPrepared: true, err: "goqu: driver.Valuer *sqlgen_test.secondCallValuer: valuer failed", conflictCalled: true},
	} {
		conflictValuer = &secondCallValuer{firstErr: errValuerFailed}
		b = sb.NewSQLBuilder(tc.isPrepared)
		sqlgen.NewInsertSQLGenerator("test", "db", tc.opts).Generate(b, exp.NewInsertClauses().
			SetInto(exp.NewIdentifierExpression("", "test", "")).
			SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
			SetFrom(newTestAppendableExpression(`select c, d from test2`, nil, valuerError{"select failed"}, nil)).
			SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"a": conflictValuer})))
		_, _, err = b.ToSQL()
		igs.EqualError(err, tc.err)
		igs.Equal(tc.conflictCalled, conflictValuer.called)
	}

	conflictValuer = &secondCallValuer{firstErr: errValuerFailed}
	b = sb.NewSQLBuilder(false)
	sqlgen.NewInsertSQLGenerator("test", "db", opts).Generate(b, exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", exp.NewIdentifierExpression("", "", ""))).
		SetFrom(newTestAppendableExpression(`select c from test2`, nil, nil, nil)).
		SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"a": conflictValuer})))
	_, _, err = b.ToSQL()
	igs.ErrorContains(err, "an empty identifier was encountered")
	igs.True(goqu.IsInvalidExpression(err))
	igs.False(conflictValuer.called)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcludedRowAliasReusesProbe() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	fromColsIc := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetFrom(newTestAppendableExpression(`select c, d from test2`, nil, nil, nil))
	withExcluded := func() exp.ConflictUpdateExpression {
		return exp.NewDoUpdateConflictExpression("a", exp.Record{
			"a": &secondCallValuer{secondValue: "second"},
			"b": exp.NewExcludedExpression("b"),
		})
	}
	withoutExcluded := func() exp.ConflictUpdateExpression {
		return exp.NewDoUpdateConflictExpression("a", exp.Record{
			"a": &secondCallValuer{secondValue: exp.NewExcludedExpression("a")},
		})
	}
	opts := sqlgen.DefaultDialectOptions()
	opts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{
			clause: ic.SetOnConflict(withExcluded()),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "excluded" ` +
				`ON CONFLICT (a) DO UPDATE SET "a"='first',"b"="excluded"."b"`,
		},
		insertTestCase{
			clause: ic.SetOnConflict(withExcluded()),
			sql: `INSERT INTO "test" ("a", "b") VALUES (?, ?) AS "excluded" ` +
				`ON CONFLICT (a) DO UPDATE SET "a"=?,"b"="excluded"."b"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1", "first"},
		},
		insertTestCase{
			clause: fromColsIc.SetOnConflict(withoutExcluded()),
			sql:    `INSERT INTO "test" ("a", "b") select c, d from test2 ON CONFLICT (a) DO UPDATE SET "a"='first'`,
		},
		insertTestCase{
			clause: fromColsIc.SetOnConflict(withExcluded()),
			sql: `INSERT INTO "test" ("a", "b") SELECT * FROM (select c, d from test2) AS "excluded" ("a", "b") ` +
				`ON CONFLICT (a) DO UPDATE SET "a"='first',"b"="excluded"."b"`,
		},
	)
}

func TestNumberedRowAliasInsertSelectPlaceholders(t *testing.T) {
	opts := sqlgen.DefaultDialectOptions()
	opts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	opts.IncludePlaceholderNum = true
	opts.PlaceHolderFragment = []byte("$")
	goqu.RegisterDialect("numbered-row-alias", "db", opts)
	defer goqu.DeregisterDialect("numbered-row-alias")
	d := goqu.Dialect("numbered-row-alias")
	sql, args, err := d.Insert("t").
		With("w", d.From("x").Where(goqu.C("k").Eq(1))).
		Cols("a", "b").
		FromQuery(d.From("s").Where(goqu.C("c").Gt(2))).
		OnConflict(goqu.DoUpdate("a", goqu.Record{"b": goqu.Excluded("b").Add(3)})).
		Prepared(true).
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `WITH w AS (SELECT * FROM "x" WHERE ("k" = $1)) INSERT INTO "t" ("a", "b") `+
		`SELECT * FROM (SELECT * FROM "s" WHERE ("c" > $2)) AS "excluded" ("a", "b") `+
		`ON CONFLICT (a) DO UPDATE SET "b"=("excluded"."b" + $3)`, sql)
	assert.Equal(t, []interface{}{int64(1), int64(2), int64(3)}, args)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcludedRowAliasOptions() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})

	numberedOpts := sqlgen.DefaultDialectOptions()
	numberedOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	numberedOpts.IncludePlaceholderNum = true
	numberedOpts.PlaceHolderFragment = []byte("$")
	b := sb.NewSQLBuilder(true)
	sqlgen.NewInsertSQLGenerator("test", "db", numberedOpts).Generate(b, ic.SetOnConflict(
		exp.NewDoUpdateConflictExpression("a", exp.Record{"a": &secondCallValuer{secondValue: exp.NewExcludedExpression("a")}}),
	))
	_, _, err := b.ToSQL()
	igs.EqualError(err, "goqu: Excluded rendered differently on a second pass; "+
		"Valuers and expressions in DoUpdate must render deterministically")
	igs.True(goqu.IsInvalidExpression(err))
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", numberedOpts),
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": "x",
				"b": exp.NewExcludedExpression("b"),
			})),
			sql: `INSERT INTO "test" ("a", "b") VALUES ($1, $2) AS "excluded" ` +
				`ON CONFLICT (a) DO UPDATE SET "a"=$3,"b"="excluded"."b"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1", "x"},
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": &secondCallValuer{secondValue: exp.NewExcludedExpression("a")},
			})),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "a"='first'`,
		},
	)

	emptyAliasOpts := sqlgen.DefaultDialectOptions()
	emptyAliasOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	emptyAliasOpts.ExcludedRowAlias = ""
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", emptyAliasOpts),
		insertTestCase{
			clause: ic.SetOnConflict(excludedB),
			err:    "goqu: dialect has no ExcludedRowAlias for Excluded, set an insert alias with As [dialect=test]",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"})),
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"='x'`,
		},
		insertTestCase{
			clause: ic.SetAlias(exp.NewIdentifierExpression("", "new", nil)).SetOnConflict(excludedB),
			sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "new" ` +
				`ON CONFLICT (a) DO UPDATE SET "b"="new"."b"`,
		},
	)

	b = sb.NewSQLBuilder(false)
	sqlgen.NewInsertSQLGenerator("test", "db", emptyAliasOpts).Generate(b, ic.SetOnConflict(excludedB))
	_, _, err = b.ToSQL()
	igs.True(goqu.IsUnsupported(err))

	columnAliasIc := ic.SetAlias(exp.ParseIdentifier("new")).SetOnConflict(excludedB)
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", numberedOpts),
		insertTestCase{
			clause: columnAliasIc,
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') AS "new" ON CONFLICT (a) DO UPDATE SET "b"="new"."b"`,
		},
		insertTestCase{
			clause:     columnAliasIc,
			sql:        `INSERT INTO "test" ("a", "b") VALUES ($1, $2) AS "new" ON CONFLICT (a) DO UPDATE SET "b"="new"."b"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1"},
		},
		insertTestCase{
			clause: ic.SetAlias(exp.ParseIdentifier("new")).SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{
				"a": &secondCallValuer{secondValue: exp.NewExcludedExpression("a")},
			})),
			sql:        `INSERT INTO "test" ("a", "b") VALUES ($1, $2) AS "new" ON CONFLICT (a) DO UPDATE SET "a"="new"."a"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1"},
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictNotSupported() {
	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsOnConflict = false
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a")).
		SetVals([][]interface{}{{"a1"}})
	isg := sqlgen.NewInsertSQLGenerator("test", "db", opts)
	igs.assertCases(isg,
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoNothingConflictExpression()),
			err:    "goqu: dialect does not support upsert [dialect=test]",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("", exp.Record{"a": "b"})),
			err:    "goqu: dialect does not support upsert [dialect=test]",
		},
	)
	igs.assertCases(isg, insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a") VALUES ('a1')`})

	noUpsertOpts := sqlgen.DefaultDialectOptions()
	noUpsertOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	noUpsertOpts.ExcludedRowAlias = ""
	noUpsertOpts.SupportsOnConflict = false
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", noUpsertOpts),
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"a": exp.NewExcludedExpression("a")})),
			err:    "goqu: dialect does not support upsert [dialect=test]",
		},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withCommonTables() {
	opts := sqlgen.DefaultDialectOptions()
	opts.WithFragment = []byte("with ")
	opts.RecursiveFragment = []byte("recursive ")

	tse := newTestAppendableExpression("select * from foo", emptyArgs, nil, nil)

	ic := exp.NewInsertClauses().SetInto(exp.NewIdentifierExpression("", "test_cte", ""))
	icCte1 := ic.CommonTablesAppend(exp.NewCommonTableExpression(false, "test_cte", tse))
	icCte2 := ic.CommonTablesAppend(exp.NewCommonTableExpression(true, "test_cte", tse))

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{
			clause: icCte1,
			sql:    `with test_cte AS (select * from foo) INSERT INTO "test_cte" DEFAULT VALUES`,
		},
		insertTestCase{
			clause:     icCte1,
			sql:        `with test_cte AS (select * from foo) INSERT INTO "test_cte" DEFAULT VALUES`,
			isPrepared: true,
		},

		insertTestCase{
			clause: icCte2,
			sql:    `with recursive test_cte AS (select * from foo) INSERT INTO "test_cte" DEFAULT VALUES`,
		},
		insertTestCase{
			clause:     icCte2,
			sql:        `with recursive test_cte AS (select * from foo) INSERT INTO "test_cte" DEFAULT VALUES`,
			isPrepared: true,
		},
	)

	opts.SupportsWithCTE = false
	expectedErr := "goqu: dialect does not support CTE WITH clause [dialect=test]"
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{clause: icCte1, err: expectedErr},
		insertTestCase{clause: icCte1, err: expectedErr, isPrepared: true},

		insertTestCase{clause: icCte2, err: expectedErr},
		insertTestCase{clause: icCte2, err: expectedErr, isPrepared: true},
	)

	opts.SupportsWithCTE = true
	opts.SupportsWithCTERecursive = false
	expectedErr = "goqu: dialect does not support CTE WITH RECURSIVE clause [dialect=test]"
	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", opts),
		insertTestCase{
			clause: icCte1,
			sql:    `with test_cte AS (select * from foo) INSERT INTO "test_cte" DEFAULT VALUES`,
		},
		insertTestCase{
			clause:     icCte1,
			sql:        `with test_cte AS (select * from foo) INSERT INTO "test_cte" DEFAULT VALUES`,
			isPrepared: true,
		},

		insertTestCase{clause: icCte2, err: expectedErr},
		insertTestCase{clause: icCte2, err: expectedErr, isPrepared: true},
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_withReturning() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{
			{"a1", "b1"},
		}).
		SetReturning(exp.NewColumnListExpression(nil, "db", "a", "b"))

	igs.assertCases(
		sqlgen.NewInsertSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') RETURNING "a", "b"`},
		insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a", "b") VALUES (?, ?) RETURNING "a", "b"`, isPrepared: true, args: []interface{}{
			"a1", "b1",
		}},
	)
}

func TestInsertSQLGenerator(t *testing.T) {
	suite.Run(t, new(insertSQLGeneratorSuite))
}
