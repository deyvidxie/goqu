package exec

import (
	"errors"

	"github.com/doug-martin/goqu/v9/internal/scantype"
)

type UnsupportedScanTypeError = scantype.UnsupportedScanTypeError

func NewUnsupportedScanTypeError(format string, args ...interface{}) error {
	return scantype.NewUnsupportedScanTypeError(format, args...)
}

func IsUnsupportedScanType(err error) bool {
	var target UnsupportedScanTypeError
	return errors.As(err, &target)
}
