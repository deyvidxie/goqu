package goqu_test

type testError struct{ msg string }

func (e testError) Error() string { return e.msg }

func newTestError(msg string) error { return testError{msg: msg} }
