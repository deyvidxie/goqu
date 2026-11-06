package exec

import (
	"errors"

	"github.com/doug-martin/goqu/v9/internal/scantype"
)

type UnsupportedScanTypeError = scantype.UnsupportedScanTypeError

func IsUnsupportedScanType(err error) bool {
	var target UnsupportedScanTypeError
	return errors.As(err, &target)
}
