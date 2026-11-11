package goqu_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidUTF8IsNotInterpolated(t *testing.T) {
	invalid := []byte{0x61, 0xff, 0xfe}
	for name, ds := range map[string]*goqu.SelectDataset{
		"bytes":  goqu.From("t").Where(goqu.C("b").Eq(invalid)),
		"string": goqu.From("t").Where(goqu.C("b").Eq("a\xff")),
	} {
		_, _, err := ds.ToSQL()
		assert.True(t, goqu.IsInvalidExpression(err), name)
	}

	sql, args, err := goqu.From("t").Prepared(true).Where(goqu.C("b").Eq(invalid)).ToSQL()
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "t" WHERE ("b" = ?)`, sql)
	assert.Equal(t, []interface{}{invalid}, args)
}
