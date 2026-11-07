package scantype

import "github.com/doug-martin/goqu/v9/internal/errors"

type UnsupportedScanTypeError = errors.UnsupportedScanTypeError

func NewUnsupportedScanTypeError(format string, args ...interface{}) error {
	return errors.NewUnsupportedScanTypeError(format, args...)
}
