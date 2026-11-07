package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewRecordFromStruct(t *testing.T) {
	type row struct {
		A int `db:"a"`
	}
	r, err := exp.NewRecordFromStruct(&row{A: 1}, true, false, "db")
	assert.NoError(t, err)
	assert.Equal(t, exp.Record{"a": 1}, r)

	r, err = exp.NewRecordFromStruct(row{A: 2}, true, false, "db")
	assert.NoError(t, err)
	assert.Equal(t, exp.Record{"a": 2}, r)

	for _, c := range []struct {
		in   interface{}
		want string
	}{
		{(*row)(nil), "goqu: record source must not be nil, got *exp_test.row"},
		{nil, "goqu: record source must not be nil, got <nil>"},
		{1, "goqu: cannot create a record from int, a struct is required"},
	} {
		_, err = exp.NewRecordFromStruct(c.in, true, false, "db")
		assert.EqualError(t, err, c.want)
		assert.True(t, errors.IsInvalidExpression(err), "%v", err)
	}
}
