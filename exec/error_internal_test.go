package exec

import (
	"reflect"
	"testing"

	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestScanErrorsAreUnsupportedScanType(t *testing.T) {
	for _, err := range []error{
		errScanStructTarget(1),
		errScanSliceTarget(1),
		errScanValTarget(1),
		errScanValSliceTarget(&[]int{}),
		errNoFieldForColumn("c"),
		errScannerFieldNeedsValue("c", reflect.TypeFor[int]()),
	} {
		assert.True(t, errors.IsUnsupportedScanType(err), "%v", err)
	}
}
