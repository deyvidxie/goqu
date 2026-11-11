package goqu_test

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type (
	insertTestCase struct {
		ds      *goqu.InsertDataset
		clauses exp.InsertClauses
	}
	insertDatasetSuite struct {
		suite.Suite
	}
)

func (ids *insertDatasetSuite) assertCases(cases ...insertTestCase) {
	for _, s := range cases {
		ids.Equal(s.clauses, s.ds.GetClauses())
	}
}

func (ids *insertDatasetSuite) TestInsert() {
	ds := goqu.Insert("test")
	ids.IsType(&goqu.InsertDataset{}, ds)
	ids.Implements((*exp.Expression)(nil), ds)
	ids.Implements((*exp.AppendableExpression)(nil), ds)
}

func (ids *insertDatasetSuite) TestClone() {
	ds := goqu.Insert("test")
	ids.Equal(ds.Clone(), ds)
}

func (ids *insertDatasetSuite) TestExpression() {
	ds := goqu.Insert("test")
	ids.Equal(ds.Expression(), ds)
}

func (ids *insertDatasetSuite) TestDialect() {
	ds := goqu.Insert("test")
	ids.NotNil(ds.Dialect())
}

func (ids *insertDatasetSuite) TestWithDialect() {
	ds := goqu.Insert("test")
	md := new(mocks.SQLDialect)
	ds = ds.SetDialect(md)

	dialect := goqu.GetDialect("default")
	dialectDs := ds.WithDialect("default")
	ids.Equal(md, ds.Dialect())
	ids.Equal(dialect, dialectDs.Dialect())
}

func (ids *insertDatasetSuite) TestPrepared() {
	ds := goqu.Insert("test")
	preparedDs := ds.Prepared(true)
	ids.True(preparedDs.IsPrepared())
	ids.False(ds.IsPrepared())
	// should apply the prepared to any datasets created from the root
	ids.True(preparedDs.Returning(goqu.C("col")).IsPrepared())

	defer goqu.SetDefaultPrepared(false)
	goqu.SetDefaultPrepared(true)

	// should be prepared by default
	ds = goqu.Insert("test")
	ids.True(ds.IsPrepared())
}

func (ids *insertDatasetSuite) TestGetClauses() {
	ds := goqu.Insert("test")
	ce := exp.NewInsertClauses().SetInto(goqu.I("test"))
	ids.Equal(ce, ds.GetClauses())
}

func (ids *insertDatasetSuite) TestWith() {
	from := goqu.From("cte")
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds: bd.With("test-cte", from),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				CommonTablesAppend(exp.NewCommonTableExpression(false, "test-cte", from)),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestWithRecursive() {
	from := goqu.From("cte")
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds: bd.WithRecursive("test-cte", from),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				CommonTablesAppend(exp.NewCommonTableExpression(true, "test-cte", from)),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestInto() {
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds:      bd.Into("items2"),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items2")),
		},
		insertTestCase{
			ds:      bd.Into(goqu.L("items2")),
			clauses: exp.NewInsertClauses().SetInto(goqu.L("items2")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)

	ids.PanicsWithError("goqu: unsupported table type bool, a string or expression is required", func() {
		bd.Into(true)
	})
}

func (ids *insertDatasetSuite) TestCols() {
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds: bd.Cols("a", "b"),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")),
		},
		insertTestCase{
			ds: bd.Cols("a", "b").Cols("c", "d"),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetCols(exp.NewColumnListExpression(nil, "db", "c", "d")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestClearCols() {
	bd := goqu.Insert("items").Cols("a", "b")
	ids.assertCases(
		insertTestCase{
			ds:      bd.ClearCols(),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")),
		},
	)
}

func (ids *insertDatasetSuite) TestColsAppend() {
	bd := goqu.Insert("items").Cols("a")
	ids.assertCases(
		insertTestCase{
			ds:      bd.ColsAppend("b"),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetCols(exp.NewColumnListExpression(nil, "db", "a")),
		},
	)
}

func (ids *insertDatasetSuite) TestFromQuery() {
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds: bd.FromQuery(goqu.From("other_items").Where(goqu.C("b").Gt(10))),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetFrom(goqu.From("other_items").Where(goqu.C("b").Gt(10))),
		},
		insertTestCase{
			ds: bd.FromQuery(goqu.From("other_items").Where(goqu.C("b").Gt(10))).Cols("a", "b"),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).
				SetFrom(goqu.From("other_items").Where(goqu.C("b").Gt(10))),
		},
		insertTestCase{
			ds:      bd.FromQuery(goqu.From("other_items")).FromQuery(nil),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestFromQueryDialectInheritance() {
	md := new(mocks.SQLDialect)
	md.On("Dialect").Return("dialect")

	ids.Run("ok, default dialect is replaced with insert dialect", func() {
		bd := goqu.Insert("items").SetDialect(md).FromQuery(goqu.From("other_items"))
		ids.Require().Equal(md, bd.GetClauses().From().(*goqu.SelectDataset).Dialect())
	})

	ids.Run("ok, insert and select dialects coincide", func() {
		bd := goqu.Insert("items").SetDialect(md).FromQuery(goqu.From("other_items").SetDialect(md))
		ids.Require().Equal(md, bd.GetClauses().From().(*goqu.SelectDataset).Dialect())
	})

	ids.Run("ok, insert and select dialects are default", func() {
		bd := goqu.Insert("items").FromQuery(goqu.From("other_items"))
		ids.Require().Equal(goqu.GetDialect("default"), bd.GetClauses().From().(*goqu.SelectDataset).Dialect())
	})

	ids.Run("panic, insert and select dialects are different", func() {
		defer func() {
			r := recover()
			if r == nil {
				ids.Fail("there should be a panic")
			}
			ids.Require().EqualError(
				r.(error),
				"goqu: incompatible dialects for INSERT (\"dialect\") and SELECT (\"other_dialect\")",
			)
			ids.True(goqu.IsInvalidExpression(r.(error)))
		}()

		otherDialect := new(mocks.SQLDialect)
		otherDialect.On("Dialect").Return("other_dialect")
		goqu.Insert("items").SetDialect(md).FromQuery(goqu.From("otherItems").SetDialect(otherDialect))
	})
}

func (ids *insertDatasetSuite) TestVals() {
	val1 := []interface{}{
		"a", "b",
	}
	val2 := []interface{}{
		"c", "d",
	}

	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds: bd.Vals(val1),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetVals([][]interface{}{val1}),
		},
		insertTestCase{
			ds: bd.Vals(val1, val2),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetVals([][]interface{}{val1, val2}),
		},
		insertTestCase{
			ds: bd.Vals(val1).Vals(val2),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetVals([][]interface{}{val1, val2}),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestClearVals() {
	val := []interface{}{
		"a", "b",
	}
	bd := goqu.Insert("items").Vals(val)
	ids.assertCases(
		insertTestCase{
			ds:      bd.ClearVals(),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetVals([][]interface{}{val}),
		},
	)
}

func (ids *insertDatasetSuite) TestRows() {
	type item struct {
		CreatedAt *time.Time `db:"created_at"`
	}
	n := time.Now()
	r := item{CreatedAt: nil}
	r2 := item{CreatedAt: &n}
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds:      bd.Rows(r),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetRows([]interface{}{r}),
		},
		insertTestCase{
			ds:      bd.Rows(r).Rows(r2),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetRows([]interface{}{r2}),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestClearRows() {
	type item struct {
		CreatedAt *time.Time `db:"created_at"`
	}
	r := item{CreatedAt: nil}
	bd := goqu.Insert("items").Rows(r)
	ids.assertCases(
		insertTestCase{
			ds:      bd.ClearRows(),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetRows([]interface{}{r}),
		},
	)
}

func (ids *insertDatasetSuite) TestRows_withRowLists() {
	type row struct {
		A int `db:"a"`
	}
	want, _, err := goqu.Insert("t").Rows([]row{{A: 1}, {A: 2}}).ToSQL()
	ids.Require().NoError(err)
	for name, c := range map[string]struct {
		rows interface{}
		want string
	}{
		"array":            {rows: [2]row{{A: 1}, {A: 2}}, want: want},
		"pointer to array": {rows: &[2]row{{A: 1}, {A: 2}}, want: want},
		"pointer to slice": {rows: &[]row{{A: 1}, {A: 2}}, want: want},
	} {
		sql, _, err := goqu.Insert("t").Rows(c.rows).ToSQL()
		ids.NoError(err, name)
		ids.Equal(c.want, sql, name)
	}
}

func (ids *insertDatasetSuite) TestEmptySources() {
	type row struct {
		A int `db:"a"`
	}
	type skipped struct {
		ID int `db:"id" goqu:"skipinsert"`
	}
	type omitted struct {
		A int `db:"a" goqu:"omitempty"`
	}
	const (
		noRows      = "goqu: Rows requires at least one row"
		emptyRows   = "goqu: Rows requires at least one row"
		emptyVals   = "goqu: Vals requires at least one row"
		emptyCols   = "goqu: Cols requires at least one column; use ClearCols to drop Cols"
		colsOnly    = "goqu: Cols without Vals or FromQuery found when generating insert sql"
		emptyValRow = "goqu: empty Vals row without Cols found when generating insert sql"
	)
	for name, c := range map[string]struct {
		ds  *goqu.InsertDataset
		err string
	}{
		"empty slice":                 {goqu.Insert("t").Rows([]row{}), noRows},
		"empty array":                 {goqu.Insert("t").Rows([0]row{}), noRows},
		"pointer to empty slice":      {goqu.Insert("t").Rows(&[]goqu.Record{}), noRows},
		"no rows":                     {goqu.Insert("t").Rows(), emptyRows},
		"empty record":                {goqu.Insert("t").Rows(goqu.Record{}), "goqu: insert rows have no columns"},
		"skipinsert-only struct":      {goqu.Insert("t").Rows(skipped{ID: 1}), "goqu: insert rows have no columns"},
		"omitempty zero struct":       {goqu.Insert("t").Rows(omitted{}), "goqu: insert rows have no columns"},
		"no vals":                     {goqu.Insert("t").Vals(), emptyVals},
		"vals then empty Vals()":      {goqu.Insert("t").Vals(goqu.Vals{1}).Vals(), emptyVals},
		"ColsAppend()":                {goqu.Insert("t").Cols("a").ColsAppend(), "goqu: ColsAppend requires at least one column"},
		"ColsAppend() then ClearCols": {goqu.Insert("t").Cols("a").ColsAppend().ClearCols(), "goqu: ColsAppend requires at least one column"},
		"empty cols":                  {goqu.Insert("t").Cols(), emptyCols},
		"empty cols with vals":        {goqu.Insert("t").Cols().Vals(goqu.Vals{1}), emptyCols},
		"empty cols with query":       {goqu.Insert("t").Cols().FromQuery(goqu.From("s")), emptyCols},
		"cols only":                   {goqu.Insert("t").Cols("a", "b"), colsOnly},
		"cols after ClearVals":        {goqu.Insert("t").Cols("a").Vals(goqu.Vals{1}).ClearVals(), colsOnly},
		"empty vals row, no cols":     {goqu.Insert("t").Vals(goqu.Vals{}), emptyValRow},
	} {
		_, _, err := c.ds.ToSQL()
		ids.EqualError(err, c.err, name)
		ids.True(goqu.IsInvalidExpression(err), name)
	}
}

func (ids *insertDatasetSuite) TestMultipleSources() {
	for name, ds := range map[string]*goqu.InsertDataset{
		"vals and query": goqu.Insert("t").Vals(goqu.Vals{1}).FromQuery(goqu.From("s")),
		"rows and query": goqu.Insert("t").Rows(goqu.Record{"a": 1}).FromQuery(goqu.From("s")),
		"rows and vals":  goqu.Insert("t").Rows(goqu.Record{"a": 1}).Vals(goqu.Vals{1}),
	} {
		_, _, err := ds.ToSQL()
		ids.EqualError(err, "goqu: more than one of Rows, Vals and FromQuery found when generating insert sql", name)
		ids.True(goqu.IsInvalidExpression(err), name)
	}
}

func (ids *insertDatasetSuite) TestDefaultValuesSources() {
	for name, ds := range map[string]*goqu.InsertDataset{
		"no source":    goqu.Insert("t"),
		"cleared rows": goqu.Insert("t").Rows([]goqu.Record{}).ClearRows(),
		"cleared vals": goqu.Insert("t").Vals(goqu.Vals{1}).ClearVals(),
		"cleared cols": goqu.Insert("t").Cols("a").ClearCols(),
	} {
		sql, _, err := ds.ToSQL()
		ids.NoError(err, name)
		ids.Equal(`INSERT INTO "t" DEFAULT VALUES`, sql, name)
	}
}

func (ids *insertDatasetSuite) TestValsWithoutCols() {
	ids.assertCases(
		insertTestCase{
			ds:      goqu.Insert("t").Vals(goqu.Vals{1, "a"}),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("t")).SetVals([][]interface{}{{1, "a"}}),
		},
	)
	for _, c := range []struct {
		ds   *goqu.InsertDataset
		sql  string
		args []interface{}
	}{
		{ds: goqu.Insert("t").Vals(goqu.Vals{1, "a"}), sql: `INSERT INTO "t" VALUES (1, 'a')`},
		{ds: goqu.Insert("t").Vals(goqu.Vals{1, "a"}, goqu.Vals{2, "b"}), sql: `INSERT INTO "t" VALUES (1, 'a'), (2, 'b')`},
		{
			ds:   goqu.Insert("t").Prepared(true).Vals(goqu.Vals{1, "a"}, goqu.Vals{2, "b"}),
			sql:  `INSERT INTO "t" VALUES (?, ?), (?, ?)`,
			args: []interface{}{int64(1), "a", int64(2), "b"},
		},
		{ds: goqu.Dialect("mysql").Insert("t").Vals(goqu.Vals{1}), sql: "INSERT INTO `t` VALUES (1)"},
	} {
		sql, args, err := c.ds.ToSQL()
		ids.NoError(err, c.sql)
		ids.Equal(c.sql, sql)
		if c.args != nil {
			ids.Equal(c.args, args, c.sql)
		} else {
			ids.Empty(args, c.sql)
		}
	}
	_, _, err := goqu.Insert("t").Vals(goqu.Vals{1, "a"}, goqu.Vals{2}).ToSQL()
	ids.EqualError(err, "goqu: rows with different value length, expected 2 got 1")
	ids.True(goqu.IsInvalidExpression(err), "%v", err)
}

func (ids *insertDatasetSuite) TestOnConflict() {
	du := goqu.DoUpdate("other_items", goqu.Record{"a": 1})

	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds:      bd.OnConflict(nil),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
		insertTestCase{
			ds:      bd.OnConflict(goqu.DoNothing()),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetOnConflict(goqu.DoNothing()),
		},
		insertTestCase{
			ds: bd.OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "items"))),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetOnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "items"))),
		},
		insertTestCase{
			ds:      bd.OnConflict(du),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetOnConflict(du),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestAs() {
	du := goqu.DoUpdate("other_items", goqu.Record{"new.a": 1})

	bd := goqu.Insert("items").As("new")
	ids.assertCases(
		insertTestCase{
			ds: bd.OnConflict(nil),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).
				SetAlias(exp.NewIdentifierExpression("", "new", "")),
		},
		insertTestCase{
			ds: bd.OnConflict(goqu.DoNothing()),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).
				SetAlias(exp.NewIdentifierExpression("", "new", "")).
				SetOnConflict(goqu.DoNothing()),
		},
		insertTestCase{
			ds: bd.OnConflict(du),
			clauses: exp.NewInsertClauses().
				SetAlias(exp.NewIdentifierExpression("", "new", "")).
				SetInto(goqu.C("items")).SetOnConflict(du),
		},
		insertTestCase{
			ds: bd,
			clauses: exp.NewInsertClauses().
				SetAlias(exp.NewIdentifierExpression("", "new", "")).
				SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestClearOnConflict() {
	du := goqu.DoUpdate("other_items", goqu.Record{"a": 1})

	bd := goqu.Insert("items").OnConflict(du)
	ids.assertCases(
		insertTestCase{
			ds:      bd.ClearOnConflict(),
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")).SetOnConflict(du),
		},
	)
}

func (ids *insertDatasetSuite) TestReturning_withTypeOnlyStructSource() {
	sql, _, err := goqu.Insert("t").Rows(goqu.Record{"a": 1}).Returning((*dsTestActionItem)(nil)).ToSQL()
	ids.NoError(err)
	ids.Equal(`INSERT INTO "t" ("a") VALUES (1) RETURNING "address", "name"`, sql)
}

func (ids *insertDatasetSuite) TestReturning() {
	bd := goqu.Insert("items")
	ids.assertCases(
		insertTestCase{
			ds: bd.Returning("a"),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db", "a")),
		},
		insertTestCase{
			ds: bd.Returning(),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db")),
		},
		insertTestCase{
			ds: bd.Returning(nil),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db")),
		},
		insertTestCase{
			ds: bd.Returning(),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db")),
		},
		insertTestCase{
			ds: bd.Returning("a").Returning("b"),
			clauses: exp.NewInsertClauses().
				SetInto(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db", "b")),
		},
		insertTestCase{
			ds:      bd,
			clauses: exp.NewInsertClauses().SetInto(goqu.C("items")),
		},
	)
}

func (ids *insertDatasetSuite) TestReturnsColumns() {
	ds := goqu.Insert("test")
	ids.False(ds.ReturnsColumns())
	ids.True(ds.Returning("foo", "bar").ReturnsColumns())
}

func (ids *insertDatasetSuite) TestExecutor() {
	mDB, _, err := sqlmock.New()
	ids.NoError(err)

	ds := goqu.New("mock", mDB).Insert("items").
		Rows(goqu.Record{"address": "111 Test Addr", "name": "Test1"})

	isql, args, err := ds.Executor().ToSQL()
	ids.NoError(err)
	ids.Empty(args)
	ids.Equal(`INSERT INTO "items" ("address", "name") VALUES ('111 Test Addr', 'Test1')`, isql)

	isql, args, err = ds.Prepared(true).Executor().ToSQL()
	ids.NoError(err)
	ids.Equal([]interface{}{"111 Test Addr", "Test1"}, args)
	ids.Equal(`INSERT INTO "items" ("address", "name") VALUES (?, ?)`, isql)

	defer goqu.SetDefaultPrepared(false)
	goqu.SetDefaultPrepared(true)

	isql, args, err = ds.Executor().ToSQL()
	ids.NoError(err)
	ids.Equal([]interface{}{"111 Test Addr", "Test1"}, args)
	ids.Equal(`INSERT INTO "items" ("address", "name") VALUES (?, ?)`, isql)
}

func (ids *insertDatasetSuite) TestInsertStruct() {
	defer goqu.SetIgnoreUntaggedFields(false)

	mDB, _, err := sqlmock.New()
	ids.NoError(err)

	item := dsUntaggedTestActionItem{
		Address:  "111 Test Addr",
		Name:     "Test1",
		Untagged: "Test2",
	}

	ds := goqu.New("mock", mDB).Insert("items").
		Rows(item)

	isql, args, err := ds.Executor().ToSQL()
	ids.NoError(err)
	ids.Empty(args)
	ids.Equal(`INSERT INTO "items" ("address", "name", "untagged") VALUES ('111 Test Addr', 'Test1', 'Test2')`, isql)

	isql, args, err = ds.Prepared(true).Executor().ToSQL()
	ids.NoError(err)
	ids.Equal([]interface{}{"111 Test Addr", "Test1", "Test2"}, args)
	ids.Equal(`INSERT INTO "items" ("address", "name", "untagged") VALUES (?, ?, ?)`, isql)

	goqu.SetIgnoreUntaggedFields(true)

	isql, args, err = ds.Executor().ToSQL()
	ids.NoError(err)
	ids.Empty(args)
	ids.Equal(`INSERT INTO "items" ("address", "name") VALUES ('111 Test Addr', 'Test1')`, isql)

	isql, args, err = ds.Prepared(true).Executor().ToSQL()
	ids.NoError(err)
	ids.Equal([]interface{}{"111 Test Addr", "Test1"}, args)
	ids.Equal(`INSERT INTO "items" ("address", "name") VALUES (?, ?)`, isql)
}

func (ids *insertDatasetSuite) TestToSQL() {
	md := new(mocks.SQLDialect)
	ds := goqu.Insert("test").SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToInsertSQL", builderMatching(sqlB), c).Return(nil).Once()
	insertSQL, args, err := ds.ToSQL()
	ids.Empty(insertSQL)
	ids.Empty(args)
	ids.Nil(err)
	md.AssertExpectations(ids.T())
}

func (ids *insertDatasetSuite) TestToSQL_Prepared() {
	md := new(mocks.SQLDialect)
	ds := goqu.Insert("test").SetDialect(md).Prepared(true)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(true)
	md.On("ToInsertSQL", builderMatching(sqlB), c).Return(nil).Once()
	insertSQL, args, err := ds.ToSQL()
	ids.Empty(insertSQL)
	ids.Empty(args)
	ids.Nil(err)
	md.AssertExpectations(ids.T())
}

func (ids *insertDatasetSuite) TestToSQL_ReturnedError() {
	md := new(mocks.SQLDialect)
	ds := goqu.Insert("test").SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	ee := newTestError("expected error")
	md.On("ToInsertSQL", builderMatching(sqlB), c).Run(func(args mock.Arguments) {
		args.Get(0).(sb.SQLBuilder).SetError(ee)
	}).Once()

	insertSQL, args, err := ds.ToSQL()
	ids.Empty(insertSQL)
	ids.Empty(args)
	ids.Equal(ee, err)
	md.AssertExpectations(ids.T())
}

func (ids *insertDatasetSuite) TestSetError() {
	err1 := newTestError("error #1")
	err2 := newTestError("error #2")
	err3 := newTestError("error #3")

	md := new(mocks.SQLDialect)
	ds := goqu.Insert("test").SetDialect(md)
	ds = ds.SetError(err1)
	ids.Equal(err1, ds.Error())
	sql, args, err := ds.ToSQL()
	ids.Empty(sql)
	ids.Empty(args)
	ids.Equal(err1, err)

	// Repeated SetError calls on Dataset should not overwrite the original error
	ds = ds.SetError(err2)
	ids.Equal(err1, ds.Error())
	sql, args, err = ds.ToSQL()
	ids.Empty(sql)
	ids.Empty(args)
	ids.Equal(err1, err)

	// Builder functions should not lose the error
	ds = ds.Cols("a", "b")
	ids.Equal(err1, ds.Error())
	sql, args, err = ds.ToSQL()
	ids.Empty(sql)
	ids.Empty(args)
	ids.Equal(err1, err)

	// Deeper errors inside SQL generation should still return original error
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToInsertSQL", builderMatching(sqlB), c).Run(func(args mock.Arguments) {
		args.Get(0).(sb.SQLBuilder).SetError(err3)
	}).Once()

	sql, args, err = ds.ToSQL()
	ids.Empty(sql)
	ids.Empty(args)
	ids.Equal(err1, err)
}

func TestInsertDataset(t *testing.T) {
	suite.Run(t, new(insertDatasetSuite))
}
