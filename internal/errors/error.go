package errors

import "fmt"

type Error struct {
	err string
}

func New(message string, args ...interface{}) error {
	return Error{err: "goqu: " + fmt.Sprintf(message, args...)}
}

func (e Error) Error() string {
	return e.err
}
