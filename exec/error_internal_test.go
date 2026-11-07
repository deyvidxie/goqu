package exec

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanErrorsAreUnsupportedScanType(t *testing.T) {
	for _, err := range []error{
		errUnsupportedScanStructType,
		errUnsupportedScanStructsType,
		errUnsupportedScanValsType,
		errScanValPointer,
		errScanValNonSlice,
		unableToFindFieldError("c"),
		scannerFieldNeedsValueError("c", reflect.TypeFor[int]()),
	} {
		assert.True(t, IsUnsupportedScanType(err), "%v", err)
	}
}
