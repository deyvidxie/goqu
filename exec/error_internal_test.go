package exec

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9/internal/scantype"
	"github.com/stretchr/testify/assert"
)

func TestIsUnsupportedScanType(t *testing.T) {
	err := scantype.NewUnsupportedScanTypeError("cannot scan %s", "x")
	assert.EqualError(t, err, "goqu: cannot scan x")
	assert.True(t, IsUnsupportedScanType(err))
	assert.True(t, IsUnsupportedScanType(fmt.Errorf("scanning: %w", err)))
	assert.False(t, IsUnsupportedScanType(fmt.Errorf("scanning: %w", assert.AnError)))
	assert.False(t, IsUnsupportedScanType(nil))
}
