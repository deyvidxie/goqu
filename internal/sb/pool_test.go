package sb_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/doug-martin/goqu/v9/internal/sb"
)

func TestReleaseSQLBuilder_doesNotMutateReturnedSQL(t *testing.T) {
	b := sb.NewSQLBuilder(false)
	b.WriteStrings("SELECT 1")
	sql, _, err := b.ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	sb.ReleaseSQLBuilder(b)
	for range 8 {
		other := sb.NewSQLBuilder(false)
		other.WriteStrings("XXXXXXXX")
		sb.ReleaseSQLBuilder(other)
	}
	if sql != "SELECT 1" {
		t.Fatalf("sql returned by ToSQL changed after release; got %q", sql)
	}
}

func TestReleaseSQLBuilder_dropsOversizedBuffers(t *testing.T) {
	big := sb.NewSQLBuilder(false)
	big.GrowBuffer(1 << 20)
	sb.ReleaseSQLBuilder(big)
	for range 64 {
		if sb.NewSQLBuilder(false) == big {
			t.Fatal("oversized builder was returned to the pool")
		}
	}
}

func TestReleaseSQLBuilder_doubleReleaseIsIgnored(t *testing.T) {
	b := sb.NewSQLBuilder(false)
	sb.ReleaseSQLBuilder(b)
	sb.ReleaseSQLBuilder(b)
	if sb.NewSQLBuilder(false) == sb.NewSQLBuilder(false) {
		t.Fatal("the same builder was handed out twice")
	}
}

func TestSQLBuilderPool_isSafeForConcurrentUse(t *testing.T) {
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			marker := string("abcdefgh"[g%8])
			for i := range 2000 {
				b := sb.NewSQLBuilder(i%2 == 0)
				if b.CurrentArgPosition() != 1 {
					t.Errorf("goroutine %d: reused builder has arg position %d", g, b.CurrentArgPosition())
					return
				}
				b.WriteStrings(marker).WriteInt(int64(i)).WriteArg(g, i)
				sql, args, err := b.ToSQL()
				if err != nil || sql != marker+strconv.Itoa(i) || len(args) != 2 || args[0] != g || args[1] != i {
					t.Errorf("goroutine %d: leaked state; sql=%q args=%v err=%v", g, sql, args, err)
					return
				}
				sb.ReleaseSQLBuilder(b)
			}
		}()
	}
	wg.Wait()
}
