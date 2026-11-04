package exec

import (
	"testing"

	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/stretchr/testify/assert"
)

func TestFromSQLBuilder_executorSurvivesBuilderReuse(t *testing.T) {
	b := sb.NewSQLBuilder(true)
	b.WriteStrings("SELECT * FROM t WHERE a = ?").WriteArg(1)
	e := NewQueryFactory("db", nil).FromSQLBuilder(b)

	for range 10 {
		sb.NewSQLBuilder(true).WriteStrings("DELETE FROM other WHERE b = ?").WriteArg(2)
	}

	query, args, err := e.ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "SELECT * FROM t WHERE a = ?", query)
	assert.Equal(t, []any{1}, args)
}
