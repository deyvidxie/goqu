package sqlgen_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
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
	igs.assertErrorSQL(b, `goqu: unsupported INSERT SQL fragment UpdateBeginSQLFragment`)
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

		insertTestCase{clause: bic, err: `goqu: rows with different value length expected 1 got 2`},
		insertTestCase{clause: bic, err: `goqu: rows with different value length expected 1 got 2`, isPrepared: true},
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

		insertTestCase{clause: bic, err: `goqu: rows with different value length expected 1 got 2`},
		insertTestCase{clause: bic, err: `goqu: rows with different value length expected 1 got 2`, isPrepared: true},
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

		insertTestCase{clause: icDuNil, err: sqlgen.ErrConflictUpdateValuesRequired.Error()},
		insertTestCase{clause: icDuNil, err: sqlgen.ErrConflictUpdateValuesRequired.Error(), isPrepared: true},

		insertTestCase{clause: icDuBad, err: "goqu: unsupported update interface type bool"},
		insertTestCase{clause: icDuBad, err: "goqu: unsupported update interface type bool", isPrepared: true},
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

		insertTestCase{clause: icDuNil, err: sqlgen.ErrConflictUpdateValuesRequired.Error()},
		insertTestCase{clause: icDuNil, err: sqlgen.ErrConflictUpdateValuesRequired.Error(), isPrepared: true},

		insertTestCase{clause: icDuBad, err: "goqu: unsupported update interface type bool"},
		insertTestCase{clause: icDuBad, err: "goqu: unsupported update interface type bool", isPrepared: true},
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
			err:    "goqu: dialect does not support Conflict Target clause [dialect=test]",
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

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcluded() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})
	sumB := exp.NewDoUpdateConflictExpression("a", exp.Record{
		"b": exp.NewLiteralExpression("? + ?", exp.NewIdentifierExpression("", "", "b"), exp.NewExcludedExpression("b")),
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
			sql:    `INSERT INTO "test" ("a", "b") VALUES ('a1', 'b1') ON CONFLICT (a) DO UPDATE SET "b"="b" + EXCLUDED."b"`,
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
	)
}

func (igs *insertSQLGeneratorSuite) TestGenerate_onConflictExcludedRowAlias() {
	ic := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
		SetVals([][]interface{}{{"a1", "b1"}})
	fromIc := exp.NewInsertClauses().
		SetInto(exp.NewIdentifierExpression("", "test", "")).
		SetFrom(newTestAppendableExpression(`select c, d from test2`, nil, nil, nil))
	excludedB := exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression("b")})
	sumB := exp.NewDoUpdateConflictExpression("a", exp.Record{
		"b": exp.NewLiteralExpression("? + ?", exp.NewIdentifierExpression("", "", "b"), exp.NewExcludedExpression("b")),
	})

	aliasOpts := sqlgen.DefaultDialectOptions()
	aliasOpts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	aliasOpts.ExcludedRowAlias = "incoming"
	fromColsIc := fromIc.SetCols(exp.NewColumnListExpression(nil, "db", "a", "b"))
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
				"a": [2]byte{1, 2},
				"b": exp.NewExcludedExpression("b"),
			})),
			sql: `INSERT INTO "test" ("a", "b") VALUES (?, ?) AS "incoming" ` +
				`ON CONFLICT (a) DO UPDATE SET "a"=?,"b"="incoming"."b"`,
			isPrepared: true,
			args:       []interface{}{"a1", "b1", [2]byte{1, 2}},
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
				`ON CONFLICT (a) DO UPDATE SET "b"="b" + "incoming"."b"`,
		},
		insertTestCase{
			clause: fromColsIc.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": "x"})),
			sql:    `INSERT INTO "test" ("a", "b") select c, d from test2 ON CONFLICT (a) DO UPDATE SET "b"='x'`,
		},
		insertTestCase{
			clause: fromIc.SetOnConflict(excludedB),
			err:    "goqu: dialect requires Cols to use Excluded with INSERT ... SELECT [dialect=test]",
		},
		insertTestCase{
			clause: exp.NewInsertClauses().
				SetInto(exp.NewIdentifierExpression("", "test", "")).
				SetRows([]interface{}{newTestAppendableExpression(`select c, d from test2`, nil, nil, nil)}).
				SetOnConflict(excludedB),
			err: "goqu: dialect requires Cols to use Excluded with INSERT ... SELECT [dialect=test]",
		},
		insertTestCase{
			clause: ic.SetOnConflict(exp.NewDoUpdateConflictExpression("a", exp.Record{"b": exp.NewExcludedExpression(1)})),
			err:    "goqu: Excluded requires a column name",
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
	for _, conflict := range []exp.ConflictExpression{
		exp.NewDoNothingConflictExpression(),
		exp.NewDoUpdateConflictExpression("", exp.Record{"a": "b"}),
	} {
		b := sb.NewSQLBuilder(false)
		isg.Generate(b, ic.SetOnConflict(conflict))
		_, _, err := b.ToSQL()
		igs.EqualError(err, "goqu: dialect does not support upsert [dialect=test]")
		igs.True(sqlgen.IsUnsupported(err))
	}
	igs.assertCases(isg, insertTestCase{clause: ic, sql: `INSERT INTO "test" ("a") VALUES ('a1')`})
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
