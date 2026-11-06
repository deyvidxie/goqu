package goqu_test

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
)

func TestIsUnsupportedScanType(t *testing.T) {
	err := goqu.NewUnsupportedScanTypeError("cannot scan %s", "x")
	assert.EqualError(t, err, "goqu: cannot scan x")
	assert.True(t, goqu.IsUnsupportedScanType(err))
	assert.True(t, goqu.IsUnsupportedScanType(fmt.Errorf("scanning: %w", err)))
	assert.False(t, goqu.IsUnsupportedScanType(goqu.ErrQueryFactoryNotFoundError))
	assert.False(t, goqu.IsUnsupportedScanType(nil))
}
