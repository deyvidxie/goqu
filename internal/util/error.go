package util

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/errors"
)

func errColumnMapNotStruct(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("cannot scan rows into %s, a struct is required", t)
}
