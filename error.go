package goqu

import (
	"errors"
	"fmt"
)

type (
	baseError struct {
		msg string
	}
	UnsupportedScanTypeError struct {
		baseError
	}
)

func (e baseError) Error() string {
	return "goqu: " + e.msg
}

func NewUnsupportedScanTypeError(format string, args ...interface{}) error {
	return UnsupportedScanTypeError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsUnsupportedScanType(err error) bool {
	var target UnsupportedScanTypeError
	return errors.As(err, &target)
}
