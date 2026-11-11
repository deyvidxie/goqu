package goqu_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taggedItem struct {
	ID   int64  `db:"db_id" json:"json_id"`
	Name string `db:"db_name" json:"json_name"`
}

type sqlCase struct {
	ds  interface{ ToSQL() (string, []any, error) }
	sql string
}

func assertSQLCases(t *testing.T, cases ...sqlCase) {
	t.Helper()
	for _, c := range cases {
		sql, _, err := c.ds.ToSQL()
		require.NoError(t, err)
		assert.Equal(t, c.sql, sql)
	}
}

func TestDialectWithTagName(t *testing.T) {
	d := goqu.DialectWithTagName("default", "json")
	assertSQLCases(t,
		sqlCase{ds: d.From("items").Select(&taggedItem{}), sql: `SELECT "json_id", "json_name" FROM "items"`},
		sqlCase{ds: d.Insert("items").Rows(taggedItem{ID: 1, Name: "a"}), sql: `INSERT INTO "items" ("json_id", "json_name") VALUES (1, 'a')`},
		sqlCase{ds: d.Update("items").Set(taggedItem{ID: 1, Name: "a"}), sql: `UPDATE "items" SET "json_id"=1,"json_name"='a'`},
		sqlCase{ds: d.Delete("items").Returning(&taggedItem{}), sql: `DELETE FROM "items" RETURNING "json_id", "json_name"`},
		sqlCase{
			ds:  d.Insert("items").FromQuery(d.From("other").Select(&taggedItem{})),
			sql: `INSERT INTO "items" SELECT "json_id", "json_name" FROM "other"`,
		},
	)
}

func TestWindowAndUsingStructsUseDatasetTag(t *testing.T) {
	d := goqu.DialectWithTagName("default", "json")
	byRow := goqu.W().PartitionBy(taggedItem{}).OrderBy(taggedItem{})
	assertSQLCases(t,
		sqlCase{
			ds:  d.From("items").Select(goqu.ROW_NUMBER().Over(byRow)),
			sql: `SELECT ROW_NUMBER() OVER (PARTITION BY "json_id", "json_name" ORDER BY "json_id", "json_name") FROM "items"`,
		},
		sqlCase{
			ds:  d.From("items").Window(goqu.W("w").PartitionBy(taggedItem{})).Select(goqu.ROW_NUMBER().OverName(goqu.I("w"))),
			sql: `SELECT ROW_NUMBER() OVER "w" FROM "items" WINDOW "w" AS (PARTITION BY "json_id", "json_name")`,
		},
		sqlCase{
			ds:  d.From("items").Join(goqu.T("other"), goqu.Using(taggedItem{})),
			sql: `SELECT * FROM "items" INNER JOIN "other" USING ("json_id", "json_name")`,
		},
		sqlCase{
			ds:  goqu.From("items").Join(goqu.T("other"), goqu.Using(taggedItem{})),
			sql: `SELECT * FROM "items" INNER JOIN "other" USING ("db_id", "db_name")`,
		},
	)
}

func TestSameStructDifferentTagNames(t *testing.T) {
	jsonTagged := goqu.DialectWithTagName("default", "json")
	assertSQLCases(t,
		sqlCase{ds: jsonTagged.From("items").Select(&taggedItem{}), sql: `SELECT "json_id", "json_name" FROM "items"`},
		sqlCase{ds: goqu.From("items").Select(&taggedItem{}), sql: `SELECT "db_id", "db_name" FROM "items"`},
		sqlCase{ds: goqu.Insert("items").Rows(taggedItem{ID: 1, Name: "a"}), sql: `INSERT INTO "items" ("db_id", "db_name") VALUES (1, 'a')`},
	)
}

func TestRegisteredDialectTagNameAndOptions(t *testing.T) {
	opts := goqu.DefaultDialectOptions()
	opts.QuoteRune = '`'
	goqu.RegisterDialect("json-registered", "json", opts)
	defer goqu.DeregisterDialect("json-registered")

	row := taggedItem{ID: 1, Name: "a"}
	assertSQLCases(t,
		sqlCase{
			ds:  goqu.Dialect("json-registered").Insert("items").Rows(row),
			sql: "INSERT INTO `items` (`json_id`, `json_name`) VALUES (1, 'a')",
		},
		sqlCase{ds: goqu.Dialect("json-registered").From("items").Select(&taggedItem{}), sql: "SELECT `json_id`, `json_name` FROM `items`"},
		sqlCase{ds: goqu.New("json-registered", nil).Update("items").Set(row), sql: "UPDATE `items` SET `json_id`=1,`json_name`='a'"},
		sqlCase{
			ds:  goqu.NewTx("json-registered", nil).Insert("items").Rows(row),
			sql: "INSERT INTO `items` (`json_id`, `json_name`) VALUES (1, 'a')",
		},
		sqlCase{
			ds:  goqu.Insert("items").WithDialect("json-registered").Rows(row),
			sql: "INSERT INTO `items` (`json_id`, `json_name`) VALUES (1, 'a')",
		},
		sqlCase{ds: goqu.Update("items").WithDialect("json-registered").Set(row), sql: "UPDATE `items` SET `json_id`=1,`json_name`='a'"},
		sqlCase{
			ds:  goqu.From("items").WithDialect("json-registered").Select(&taggedItem{}),
			sql: "SELECT `json_id`, `json_name` FROM `items`",
		},
		sqlCase{
			ds:  goqu.From("items").WithDialect("json-registered").Insert().Rows(row),
			sql: "INSERT INTO `items` (`json_id`, `json_name`) VALUES (1, 'a')",
		},
		sqlCase{
			ds:  goqu.Delete("items").WithDialect("json-registered").Returning(&taggedItem{}),
			sql: "DELETE FROM `items` RETURNING `json_id`, `json_name`",
		},
		sqlCase{
			ds:  goqu.From("items").Insert().WithDialect("json-registered").Rows(row),
			sql: "INSERT INTO `items` (`json_id`, `json_name`) VALUES (1, 'a')",
		},
		sqlCase{
			ds:  goqu.New("default", nil).From("items").WithDialect("json-registered").Select(&taggedItem{}),
			sql: "SELECT `db_id`, `db_name` FROM `items`",
		},
		sqlCase{
			ds:  goqu.New("default", nil).Update("items").WithDialect("json-registered").Set(row),
			sql: "UPDATE `items` SET `db_id`=1,`db_name`='a'",
		},
		sqlCase{
			ds:  goqu.New("default", nil).Delete("items").WithDialect("json-registered").Returning(&taggedItem{}),
			sql: "DELETE FROM `items` RETURNING `db_id`, `db_name`",
		},
		sqlCase{
			ds:  goqu.From("items").Update().WithDialect("json-registered").Set(row),
			sql: "UPDATE `items` SET `json_id`=1,`json_name`='a'",
		},
		sqlCase{
			ds:  goqu.From("items").Delete().WithDialect("json-registered").Returning(&taggedItem{}),
			sql: "DELETE FROM `items` RETURNING `json_id`, `json_name`",
		},
		sqlCase{
			ds:  goqu.DialectWithTagName("default", "db").Insert("items").WithDialect("json-registered").Rows(row),
			sql: "INSERT INTO `items` (`db_id`, `db_name`) VALUES (1, 'a')",
		},
		sqlCase{
			ds:  goqu.DialectWithTagName("json-registered", "db").Insert("items").Rows(row),
			sql: "INSERT INTO `items` (`db_id`, `db_name`) VALUES (1, 'a')",
		},
	)
}

func TestNew_registeredTagNameScansStructs(t *testing.T) {
	goqu.RegisterDialect("json-scan", "json", goqu.DefaultDialectOptions())
	defer goqu.DeregisterDialect("json-scan")

	mDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "json_id", "json_name" FROM "items" LIMIT 1`)).
		WillReturnRows(sqlmock.NewRows([]string{"json_id", "json_name"}).AddRow(1, "a"))

	var got taggedItem
	found, err := goqu.New("json-scan", mDB).From("items").ScanStruct(&got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, taggedItem{ID: 1, Name: "a"}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewTxWithTagName_emptyTagScansWithRegisteredTag(t *testing.T) {
	goqu.RegisterDialect("json-tx-scan", "json", goqu.DefaultDialectOptions())
	defer goqu.DeregisterDialect("json-tx-scan")

	mDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "json_id", "json_name" FROM "items" LIMIT 1`)).
		WillReturnRows(sqlmock.NewRows([]string{"json_id", "json_name"}).AddRow(1, "a"))
	tx, err := mDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	var got taggedItem
	found, err := goqu.NewTxWithTagName("json-tx-scan", "", tx).From("items").ScanStruct(&got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, taggedItem{ID: 1, Name: "a"}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabaseKeepsRegisteredEmptyTagAcrossWithDialect(t *testing.T) {
	goqu.RegisterDialect("empty-tag", "", goqu.DefaultDialectOptions())
	defer goqu.DeregisterDialect("empty-tag")

	assertSQLCases(t,
		sqlCase{
			ds:  goqu.New("empty-tag", nil).From("items").WithDialect("default").Select(&taggedItem{}),
			sql: `SELECT "id", "name" FROM "items"`,
		},
	)
}

func TestUnregisteredDialectNameIsAnError(t *testing.T) {
	const typo = "mysqll"
	d := goqu.Dialect(typo)
	for name, ds := range map[string]interface{ ToSQL() (string, []any, error) }{
		"Dialect.From":             d.From("t"),
		"Dialect.Select":           d.Select("a"),
		"Dialect.Insert":           d.Insert("t").Rows(goqu.Record{"a": 1}),
		"Dialect.Update":           d.Update("t").Set(goqu.Record{"a": 1}),
		"Dialect.Delete":           d.Delete("t"),
		"Dialect.Truncate":         d.Truncate("t"),
		"DialectWithTagName.From":  goqu.DialectWithTagName(typo, "json").From("t"),
		"New.From":                 goqu.New(typo, nil).From("t"),
		"NewWithTagName.From":      goqu.NewWithTagName(typo, "json", nil).From("t"),
		"NewTx.Insert":             goqu.NewTx(typo, nil).Insert("t").Rows(goqu.Record{"a": 1}),
		"Select.WithDialect":       goqu.From("t").WithDialect(typo),
		"Insert.WithDialect":       goqu.Insert("t").Rows(goqu.Record{"a": 1}).WithDialect(typo),
		"Update.WithDialect":       goqu.Update("t").Set(goqu.Record{"a": 1}).WithDialect(typo),
		"Delete.WithDialect":       goqu.Delete("t").WithDialect(typo),
		"Truncate.WithDialect":     goqu.Truncate("t").WithDialect(typo),
		"WithDialect then a valid": goqu.From("t").WithDialect(typo).WithDialect("default"),
		"Select.Update":            d.From("t").Update().Set(goqu.Record{"a": 1}),
		"FromQuery":                goqu.Dialect("postgres").Insert("t").FromQuery(d.From("s")),
	} {
		_, _, err := ds.ToSQL()
		assert.True(t, goqu.IsUnknownDialect(err), name)
		assert.EqualError(t, err, `goqu: dialect "mysqll" is not registered; blank-import its dialect package or call RegisterDialect`, name)
	}

	_, _, err := d.ExpressionSQL(goqu.C("a").Eq(1))
	assert.True(t, goqu.IsUnknownDialect(err))
	_, _, err = goqu.New(typo, nil).PreparedExpressionSQL(goqu.C("a").Eq(1))
	assert.True(t, goqu.IsUnknownDialect(err))
}

func TestFromQueryAcrossTagNamesOfOneDialect(t *testing.T) {
	goqu.RegisterDialect("from-query-test", "db", goqu.DefaultDialectOptions())
	defer goqu.DeregisterDialect("from-query-test")

	dbTagged := goqu.Dialect("from-query-test")
	jsonTagged := goqu.DialectWithTagName("from-query-test", "json")
	assert.NotPanics(t, func() { jsonTagged.Insert("items").FromQuery(dbTagged.From("other")) })
	assert.NotPanics(t, func() { dbTagged.Insert("items").FromQuery(jsonTagged.From("other")) })
}

func TestScanStruct_structAlreadyMappedUnderAnotherTag(t *testing.T) {
	_, _, err := goqu.From("items").Select(&taggedItem{}).ToSQL()
	require.NoError(t, err)

	mDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "json_id", "json_name" FROM "items" LIMIT 1`)).
		WillReturnRows(sqlmock.NewRows([]string{"json_id", "json_name"}).AddRow(1, "a"))

	var got taggedItem
	found, err := goqu.NewWithTagName("default", "json", mDB).From("items").ScanStruct(&got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, taggedItem{ID: 1, Name: "a"}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}
