package goqu_test

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
)

func TestIsUnknownDialect(t *testing.T) {
	_, err := goqu.LookupDialect("nope")
	assert.EqualError(t, err, `goqu: dialect "nope" is not registered; blank-import its dialect package or call RegisterDialect`)
	assert.True(t, goqu.IsUnknownDialect(err))
	assert.True(t, goqu.IsUnknownDialect(fmt.Errorf("opening: %w", err)))
	assert.False(t, goqu.IsUnknownDialect(goqu.ErrQueryFactoryNotFoundError))
	assert.False(t, goqu.IsUnknownDialect(nil))
	var target goqu.UnknownDialectError
	assert.ErrorAs(t, err, &target)
}
