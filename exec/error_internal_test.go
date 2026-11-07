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

func TestNewFailedQueryExecutorReturnsItsError(t *testing.T) {
	err := newTestError("no database")
	q := NewFailedQueryExecutor(err)
	_, _, toSQLErr := q.ToSQL()
	assert.Equal(t, err, toSQLErr)
	_, execErr := q.Exec()
	assert.Equal(t, err, execErr)
	var items []struct{ A int }
	assert.Equal(t, err, q.ScanStructs(&items))
	var vals []int
	assert.Equal(t, err, q.ScanVals(&vals))
}
