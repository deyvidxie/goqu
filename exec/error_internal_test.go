package exec

import (
	"database/sql"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

type scannerRow struct {
	A int `db:"a"`
}

func (r *scannerRow) Scan(interface{}) error { return nil }

func TestStructTargetsMustBeRows(t *testing.T) {
	type hiddenColumns struct {
		X int `db:"x"`
	}
	type unexportedEmbedRow struct {
		*hiddenColumns
		ID int `db:"id"`
	}
	for target, want := range map[interface{}]string{
		&[]sql.NullString{}: "goqu: cannot scan rows into sql.NullString, which scans as one column; " +
			"use ScanVals/ScanVal, ScanAllVals[sql.NullString]/ScanOneVal[sql.NullString] or IterVals[sql.NullString]",
		&[]time.Time{}: "goqu: cannot scan rows into time.Time, which scans as one column; " +
			"use ScanVals/ScanVal, ScanAllVals[time.Time]/ScanOneVal[time.Time] or IterVals[time.Time]",
		&[]netip.Addr{}: "goqu: cannot scan rows into netip.Addr, which has no fields goqu can scan into",
	} {
		_, err := checkScanStructsTarget(target, "db")
		assert.EqualError(t, err, want)
		assert.True(t, errors.IsUnsupportedScanType(err))
	}

	_, err := QueryExecutor{}.ScanStruct(&sql.NullString{})
	assert.EqualError(t, err, "goqu: cannot scan rows into sql.NullString, which scans as one column; "+
		"use ScanVals/ScanVal, ScanAllVals[sql.NullString]/ScanOneVal[sql.NullString] or IterVals[sql.NullString]")
	_, err = QueryExecutor{}.ScanStruct(&netip.Addr{})
	assert.EqualError(t, err, "goqu: cannot scan rows into netip.Addr, which has no fields goqu can scan into")

	_, err = checkScanStructsTarget(&[]scannerRow{}, "db")
	assert.NoError(t, err)
	_, err = checkScanStructsTarget(&[]*scannerRow{}, "db")
	assert.NoError(t, err)
	_, err = checkScanStructsTarget(&[]unexportedEmbedRow{}, "db")
	assert.NoError(t, err)

	queryErr := newTestError("query not run")
	_, err = QueryExecutor{err: queryErr}.ScanStruct(&scannerRow{})
	assert.Equal(t, queryErr, err)
	_, err = QueryExecutor{err: queryErr}.ScanStruct(&unexportedEmbedRow{})
	assert.Equal(t, queryErr, err)
}

func TestNilScanTargets(t *testing.T) {
	type row struct{ A int }
	_, err := QueryExecutor{}.ScanStruct((*row)(nil))
	assert.EqualError(t, err, "goqu: scan target must not be nil, got *exec.row")
	assert.True(t, errors.IsUnsupportedScanType(err))
	_, err = checkScanStructsTarget((*[]row)(nil), "db")
	assert.EqualError(t, err, "goqu: scan target must not be nil, got *[]exec.row")
	_, err = checkScanValsTarget(nil, "db")
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
