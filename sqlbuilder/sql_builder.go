package sqlbuilder

import (
	"bytes"
	"slices"
	"strconv"
)

const (
	decimalBase = 10
	float64Bits = 64
)

type (
	SQLBuilder interface {
		Error() error
		// SetError keeps the first error; writes after an error are no-ops.
		SetError(err error) SQLBuilder
		WriteArg(i ...any) SQLBuilder
		Write(p []byte) SQLBuilder
		WriteStrings(ss ...string) SQLBuilder
		WriteRunes(r ...rune) SQLBuilder
		// WriteInt appends the base 10 representation of i.
		WriteInt(i int64) SQLBuilder
		// WriteFloat appends f formatted with the 'f' format and the smallest precision that round trips.
		WriteFloat(f float64) SQLBuilder
		GrowArgs(n int) SQLBuilder
		GrowBuffer(n int) SQLBuilder
		IsPrepared() bool
		// CurrentArgPosition is the 1-based number of the next placeholder; WriteArg advances it by len(i).
		CurrentArgPosition() int
		// ToSQL returns non-nil args when err is nil.
		ToSQL() (sql string, args []any, err error)
	}
	sqlBuilder struct {
		buf                *bytes.Buffer
		isPrepared         bool
		currentArgPosition int
		args               []any
		err                error
		released           bool
	}
)

func New(isPrepared bool) SQLBuilder {
	return newSQLBuilder(isPrepared)
}

func newSQLBuilder(isPrepared bool) *sqlBuilder {
	return &sqlBuilder{
		buf:                &bytes.Buffer{},
		currentArgPosition: 1,
		isPrepared:         isPrepared,
	}
}

func (b *sqlBuilder) Reuse(isPrepared bool) {
	b.released = false
	b.isPrepared = isPrepared
}

// Recycle leaves the sql string and args returned from ToSQL valid.
func (b *sqlBuilder) Recycle(maxBufferCap int) bool {
	if b.released || b.buf.Cap() > maxBufferCap {
		return false
	}
	b.reset()
	b.released = true
	return true
}

func (b *sqlBuilder) reset() {
	b.buf.Reset()
	// Never reuse the args backing array: ToSQL hands it to the caller.
	b.args = nil
	b.err = nil
	b.isPrepared = false
	b.currentArgPosition = 1
}

func (b *sqlBuilder) Error() error {
	return b.err
}

func (b *sqlBuilder) SetError(err error) SQLBuilder {
	if b.err == nil {
		b.err = err
	}
	return b
}

func (b *sqlBuilder) Write(bs []byte) SQLBuilder {
	if b.err == nil {
		b.buf.Write(bs)
	}
	return b
}

func (b *sqlBuilder) WriteStrings(ss ...string) SQLBuilder {
	if b.err == nil {
		for _, s := range ss {
			b.buf.WriteString(s)
		}
	}
	return b
}

func (b *sqlBuilder) WriteRunes(rs ...rune) SQLBuilder {
	if b.err == nil {
		for _, r := range rs {
			b.buf.WriteRune(r)
		}
	}
	return b
}

func (b *sqlBuilder) WriteInt(i int64) SQLBuilder {
	if b.err == nil {
		b.buf.Write(strconv.AppendInt(b.buf.AvailableBuffer(), i, decimalBase))
	}
	return b
}

func (b *sqlBuilder) WriteFloat(f float64) SQLBuilder {
	if b.err == nil {
		b.buf.Write(strconv.AppendFloat(b.buf.AvailableBuffer(), f, 'f', -1, float64Bits))
	}
	return b
}

func (b *sqlBuilder) GrowArgs(n int) SQLBuilder {
	if b.err == nil && n > 0 {
		b.args = slices.Grow(b.args, n)
	}
	return b
}

func (b *sqlBuilder) GrowBuffer(n int) SQLBuilder {
	if b.err == nil && n > 0 {
		b.buf.Grow(n)
	}
	return b
}

func (b *sqlBuilder) IsPrepared() bool {
	return b.isPrepared
}

func (b *sqlBuilder) CurrentArgPosition() int {
	return b.currentArgPosition
}

func (b *sqlBuilder) WriteArg(i ...any) SQLBuilder {
	if b.err == nil {
		b.currentArgPosition += len(i)
		b.args = append(b.args, i...)
	}
	return b
}

func (b *sqlBuilder) ToSQL() (sql string, args []any, err error) {
	if b.err != nil {
		return sql, args, b.err
	}
	args = b.args
	if args == nil {
		args = []any{}
	}
	return b.buf.String(), args, nil
}
