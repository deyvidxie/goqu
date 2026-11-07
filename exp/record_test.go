package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
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

	_, err = exp.NewRecordFromStruct((*row)(nil), true, false, "db")
	assert.EqualError(t, err, "goqu: cannot create a record from a nil *exp_test.row")
}
