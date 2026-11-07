package exec

import "github.com/doug-martin/goqu/v9/internal/errors"

type UnsupportedScanTypeError = errors.UnsupportedScanTypeError

func IsUnsupportedScanType(err error) bool {
	return errors.IsUnsupportedScanType(err)
}
