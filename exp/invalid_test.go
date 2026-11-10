package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type invalidSourceNode struct {
	ID     int64 `db:"id"`
	Parent *invalidSourceNode
}

func TestInvalidExpression(t *testing.T) {
	cols := exp.NewColumnListExpression(nil, "db", invalidSourceNode{}).Columns()
	require.Len(t, cols, 1)
	invalid, ok := cols[0].(exp.InvalidExpression)
	require.True(t, ok)
	assert.Equal(t, invalid, invalid.Expression())
	assert.Equal(t, invalid, invalid.Clone())
	assert.EqualError(t, invalid.Err(), "goqu: cannot derive columns from exp_test.invalidSourceNode: "+
		"field Parent leads back to exp_test.invalidSourceNode; tag it `db:\"-\"`")
}
