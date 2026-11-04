package goqu_test

import (
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
		sqlCase{ds: goqu.From("items").WithDialect("json-registered").Select("a"), sql: "SELECT `a` FROM `items`"},
		sqlCase{
			ds:  goqu.DialectWithTagName("json-registered", "db").Insert("items").Rows(row),
			sql: "INSERT INTO `items` (`db_id`, `db_name`) VALUES (1, 'a')",
		},
	)
}

func TestFromQueryAcrossTagNamesOfOneDialect(t *testing.T) {
	goqu.RegisterDialect("from-query-test", "db", goqu.DefaultDialectOptions())
	defer goqu.DeregisterDialect("from-query-test")

	dbTagged := goqu.Dialect("from-query-test")
	jsonTagged := goqu.DialectWithTagName("from-query-test", "json")
	assert.NotPanics(t, func() { jsonTagged.Insert("items").FromQuery(dbTagged.From("other")) })
	assert.NotPanics(t, func() { dbTagged.Insert("items").FromQuery(jsonTagged.From("other")) })
}

func TestNewWithTagName_ScanStruct(t *testing.T) {
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
