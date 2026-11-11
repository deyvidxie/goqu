package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
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

func TestNilOrderedColumn(t *testing.T) {
	a := exp.NewIdentifierExpression("", "", "a").Asc()
	cols := exp.NewOrderedColumnList(a, nil).Clone().(exp.ColumnListExpression).Columns()
	require.Len(t, cols, 2)
	assert.Equal(t, a, cols[0])
	invalid, ok := cols[1].(exp.InvalidExpression)
	require.True(t, ok)
	assert.True(t, errors.IsInvalidExpression(invalid.Err()))
	assert.EqualError(t, invalid.Err(), "goqu: Order expressions must not be nil")
}

func TestUnsupportedAliasAndColumnTypes(t *testing.T) {
	aliased := exp.NewAliasExpression(exp.NewIdentifierExpression("", "", "a"), 1)
	invalid, ok := aliased.Clone().(exp.AliasedExpression).Aliased().(exp.InvalidExpression)
	require.True(t, ok)
	assert.EqualError(t, invalid.Err(), "goqu: unsupported alias type int, a string or identifier expression is required")
	assert.NotPanics(t, func() { aliased.Col("b") })

	cols := exp.NewColumnListExpression(nil, "db", "a", 1).Columns()
	require.Len(t, cols, 2)
	invalid, ok = cols[1].(exp.InvalidExpression)
	require.True(t, ok)
	assert.EqualError(t, invalid.Err(), "goqu: unsupported column type int, a string, expression or struct is required")
}
