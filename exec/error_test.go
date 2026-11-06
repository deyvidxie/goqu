package exec_test

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9/exec"
	"github.com/stretchr/testify/assert"
)

func TestIsUnsupportedScanType(t *testing.T) {
	err := exec.NewUnsupportedScanTypeError("cannot scan %s", "x")
	assert.EqualError(t, err, "goqu: cannot scan x")
	assert.True(t, exec.IsUnsupportedScanType(err))
	assert.True(t, exec.IsUnsupportedScanType(fmt.Errorf("scanning: %w", err)))
	assert.False(t, exec.IsUnsupportedScanType(fmt.Errorf("scanning: %w", assert.AnError)))
	assert.False(t, exec.IsUnsupportedScanType(nil))
}
