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

func TestDialectWithTagName(t *testing.T) {
	d := goqu.DialectWithTagName("default", "json")
	for _, c := range []struct {
		ds  interface{ ToSQL() (string, []any, error) }
		sql string
	}{
		{ds: d.From("items").Select(&taggedItem{}), sql: `SELECT "json_id", "json_name" FROM "items"`},
		{ds: d.Insert("items").Rows(taggedItem{ID: 1, Name: "a"}), sql: `INSERT INTO "items" ("json_id", "json_name") VALUES (1, 'a')`},
		{ds: d.Update("items").Set(taggedItem{ID: 1, Name: "a"}), sql: `UPDATE "items" SET "json_id"=1,"json_name"='a'`},
		{
			ds:  d.Insert("items").FromQuery(d.From("other").Select(&taggedItem{})),
			sql: `INSERT INTO "items" SELECT "json_id", "json_name" FROM "other"`,
		},
		{ds: goqu.From("items").Select(&taggedItem{}), sql: `SELECT "db_id", "db_name" FROM "items"`},
		{ds: goqu.Insert("items").Rows(taggedItem{ID: 1, Name: "a"}), sql: `INSERT INTO "items" ("db_id", "db_name") VALUES (1, 'a')`},
	} {
		sql, _, err := c.ds.ToSQL()
		require.NoError(t, err)
		assert.Equal(t, c.sql, sql)
	}
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
