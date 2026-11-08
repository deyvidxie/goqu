package exec

import (
	"reflect"
	"testing"

	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestNilScanTargets(t *testing.T) {
	type row struct{ A int }
	_, err := QueryExecutor{}.ScanStruct((*row)(nil))
	assert.EqualError(t, err, "goqu: scan target must not be nil, got *exec.row")
	assert.True(t, errors.IsUnsupportedScanType(err))
	_, err = checkScanStructsTarget((*[]row)(nil))
	assert.EqualError(t, err, "goqu: scan target must not be nil, got *[]exec.row")
	_, err = checkScanValsTarget(nil)
	assert.EqualError(t, err, "goqu: scan target must not be nil, got <nil>")
}

func TestScanErrorsAreUnsupportedScanType(t *testing.T) {
	for _, err := range []error{
		errScanStructTarget(1),
		errScanSliceTarget(1),
		errScanValTarget(1),
		errScanValSliceTarget(&[]int{}),
		errScannerFieldNeedsValue("c", reflect.TypeFor[int]()),
	} {
		assert.True(t, errors.IsUnsupportedScanType(err), "%v", err)
	}
}

func TestUnmappedColumnIsColumnMismatch(t *testing.T) {
	err := errNoFieldForColumn("c")
	assert.EqualError(t, err, `goqu: unable to find corresponding field to column "c" returned by query`)
	assert.True(t, errors.IsColumnMismatch(err))
	assert.False(t, errors.IsUnsupportedScanType(err))
}
