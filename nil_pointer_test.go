package goqu_test

import (
	"database/sql/driver"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type valueReceiverValuer struct{}

func (valueReceiverValuer) Value() (driver.Value, error) { return "value", nil }

type pointerReceiverValuer struct{}

func (*pointerReceiverValuer) Value() (driver.Value, error) { return "pointer", nil }

type pointerReceiverNullValuer struct{}

func (*pointerReceiverNullValuer) Value() (driver.Value, error) { return nil, nil }

func TestNilPointerValuerRendersTheSameInComparisonsAndValues(t *testing.T) {
	for _, c := range []struct {
		val         interface{}
		eq, neq, in string
	}{
		{(*valueReceiverValuer)(nil), `("a" IS NULL)`, `("a" IS NOT NULL)`, `NULL`},
		{(*pointerReceiverValuer)(nil), `("a" = 'pointer')`, `("a" != 'pointer')`, `'pointer'`},
		{(*pointerReceiverNullValuer)(nil), `("a" IS NULL)`, `("a" IS NOT NULL)`, `NULL`},
	} {
		assertSQLCases(t,
			sqlCase{ds: goqu.From("t").Where(goqu.C("a").Eq(c.val)), sql: `SELECT * FROM "t" WHERE ` + c.eq},
			sqlCase{ds: goqu.From("t").Where(goqu.Ex{"a": c.val}), sql: `SELECT * FROM "t" WHERE ` + c.eq},
			sqlCase{ds: goqu.From("t").Where(goqu.C("a").Neq(c.val)), sql: `SELECT * FROM "t" WHERE ` + c.neq},
			sqlCase{ds: goqu.Insert("t").Rows(goqu.Record{"a": c.val}), sql: `INSERT INTO "t" ("a") VALUES (` + c.in + `)`},
		)
	}
}

func TestNilSelectDatasetIsAnError(t *testing.T) {
	var nilDS *goqu.SelectDataset
	for name, ds := range map[string]interface{ ToSQL() (string, []any, error) }{
		"From":              goqu.From(nilDS),
		"Union":             goqu.From("t").Union(nilDS),
		"UnionAll":          goqu.From("t").UnionAll(nilDS),
		"Intersect":         goqu.From("t").Intersect(nilDS),
		"IntersectAll":      goqu.From("t").IntersectAll(nilDS),
		"Select With":       goqu.From("t").With("w", nilDS),
		"In":                goqu.From("t").Where(goqu.C("a").In(nilDS)),
		"Insert With":       goqu.Insert("t").With("c", nilDS).Rows(goqu.Record{"a": 1}),
		"Update With":       goqu.Update("t").With("c", nilDS).Set(goqu.Record{"a": 1}),
		"Delete With":       goqu.Delete("t").With("c", nilDS),
		"Update From":       goqu.Update("t").Set(goqu.Record{"a": 1}).From(nilDS),
		"Update Set value":  goqu.Update("t").Set(goqu.Record{"a": nilDS}),
		"Insert Rows value": goqu.Insert("t").Rows(goqu.Record{"a": nilDS}),
	} {
		var err error
		require.NotPanics(t, func() { _, _, err = ds.ToSQL() }, name)
		assert.True(t, goqu.IsInvalidExpression(err), name)
		assert.EqualError(t, err, "goqu: subquery must not be nil, got *goqu.SelectDataset", name)
	}
}
