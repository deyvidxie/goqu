package sqlbuilder //nolint:testpackage // exercises unexported reset state

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

// Ensures that after calling ToSQL(), the returned args slice is not mutated
// by subsequent calls to reset() and further writes on the same builder.
func TestSQLBuilder_ResetDoesNotMutateReturnedArgs(t *testing.T) {
	// Use the unexported constructor to deterministically operate on a single builder instance.
	sb := newSQLBuilder(false)

	sb.WriteArg("first", 2)
	_, argsBefore, err := sb.ToSQL()
	if err != nil {
		t.Fatalf("unexpected error from ToSQL: %v", err)
	}
	if len(argsBefore) != 2 {
		t.Fatalf("expected 2 args before reset, got %d", len(argsBefore))
	}
	argsSnapshot := append([]any(nil), argsBefore...)

	// If reset reused the same backing array for args, the following WriteArg calls would overwrite argsBefore.
	sb.reset()
	sb.WriteArg("new1", "new2")

	if !reflect.DeepEqual(argsBefore, argsSnapshot) {
		t.Fatalf("args returned by first ToSQL were mutated after reset+reuse; before=%v after=%v", argsSnapshot, argsBefore)
	}
}

func TestSQLBuilder_RecycleResetsState(t *testing.T) {
	b := newSQLBuilder(true)
	b.WriteStrings("SELECT ")
	b.WriteRunes('1')
	b.WriteArg(123)
	b.SetError(newTestError("assert"))

	if !b.Recycle(1 << 10) {
		t.Fatal("a small builder must be recyclable")
	}
	if got := b.buf.String(); got != "" {
		t.Fatalf("buffer not reset; got %q", got)
	}
	if b.args != nil || b.err != nil || b.isPrepared || b.currentArgPosition != 1 {
		t.Fatalf("state not reset: %+v", b)
	}
}

func TestSQLBuilder_RecycleRefusesOversizedBuffersAndSecondRecycle(t *testing.T) {
	big := newSQLBuilder(false)
	big.GrowBuffer(2 << 10)
	if big.Recycle(1 << 10) {
		t.Fatal("an oversized builder must not be recyclable")
	}
	b := newSQLBuilder(false)
	if !b.Recycle(1<<10) || b.Recycle(1<<10) {
		t.Fatal("a builder must be recyclable exactly once until it is reused")
	}
	b.Reuse(true)
	if !b.IsPrepared() || !b.Recycle(1<<10) {
		t.Fatal("Reuse must make the builder recyclable again")
	}
}

// A new builder and a reset builder must be indistinguishable (matters for equality in tests).
func TestFreshAndResetBuildersAreEqual(t *testing.T) {
	used := newSQLBuilder(false)
	used.WriteStrings("abc")
	used.WriteArg(1)
	used.reset()
	fresh := newSQLBuilder(false)
	if used.currentArgPosition != fresh.currentArgPosition || used.isPrepared != fresh.isPrepared ||
		!errors.Is(used.err, fresh.err) || !reflect.DeepEqual(used.args, fresh.args) {
		t.Fatalf("reset builder differs from a fresh one: %+v vs %+v", used, fresh)
	}
}

func TestSQLBuilder_ToSQLReturnsNonNilArgs(t *testing.T) {
	_, args, err := newSQLBuilder(true).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if args == nil || len(args) != 0 {
		t.Fatalf("expected an empty, non-nil args slice; got %#v", args)
	}
}

func TestSQLBuilder_WriteIntAndFloat(t *testing.T) {
	b := newSQLBuilder(false)
	b.WriteInt(0).WriteStrings(",").
		WriteInt(-42).WriteStrings(",").
		WriteInt(math.MaxInt64).WriteStrings(",").
		WriteInt(math.MinInt64).WriteStrings(",").
		WriteFloat(1.5).WriteStrings(",").
		WriteFloat(14000000.5).WriteStrings(",").
		WriteFloat(1e21).WriteStrings(",").
		WriteFloat(0.000001)
	sql, _, err := b.ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	expected := "0,-42,9223372036854775807,-9223372036854775808,1.5,14000000.5,1000000000000000000000,0.000001"
	if sql != expected {
		t.Fatalf("got %q, want %q", sql, expected)
	}

	// once an error is set nothing is written
	errB := newSQLBuilder(false).SetError(newTestError("boom"))
	errB.WriteInt(1).WriteFloat(2)
	if got := errB.(*sqlBuilder).buf.String(); got != "" {
		t.Fatalf("builder with an error must not write; got %q", got)
	}
}

func TestSQLBuilder_Grow(t *testing.T) {
	b := newSQLBuilder(true)
	b.WriteArg("a")
	b.GrowArgs(100)
	if cap(b.args)-len(b.args) < 100 {
		t.Fatalf("GrowArgs did not make room for 100 args: len=%d cap=%d", len(b.args), cap(b.args))
	}
	if len(b.args) != 1 || b.args[0] != "a" {
		t.Fatalf("GrowArgs must keep existing args; got %v", b.args)
	}
	b.GrowArgs(0)
	b.GrowArgs(-1)
	b.GrowBuffer(0)
	b.GrowBuffer(-5)
	b.GrowBuffer(1000)
	if b.buf.Cap() < 1000 {
		t.Fatalf("GrowBuffer did not grow the buffer; cap=%d", b.buf.Cap())
	}

	errB := newSQLBuilder(true).SetError(newTestError("boom")).(*sqlBuilder)
	errB.GrowArgs(10)
	errB.GrowBuffer(10)
	if cap(errB.args) != 0 {
		t.Fatalf("GrowArgs must be a no-op on a builder with an error; cap=%d", cap(errB.args))
	}
}
