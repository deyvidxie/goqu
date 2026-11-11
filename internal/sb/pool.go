package sb

import (
	"sync"

	"github.com/doug-martin/goqu/v9/sqlbuilder"
)

const maxPooledBufferCap = 64 << 10

type pooledBuilder interface {
	sqlbuilder.SQLBuilder
	Reuse(isPrepared bool)
	Recycle(maxBufferCap int) bool
}

var builderPool = sync.Pool{
	New: func() any {
		return sqlbuilder.New(false)
	},
}

func NewSQLBuilder(isPrepared bool) sqlbuilder.SQLBuilder {
	b, _ := builderPool.Get().(pooledBuilder)
	b.Reuse(isPrepared)
	return b
}

// ReleaseSQLBuilder returns b to the pool; b must not be used afterwards.
func ReleaseSQLBuilder(b sqlbuilder.SQLBuilder) {
	if p, ok := b.(pooledBuilder); ok && p.Recycle(maxPooledBufferCap) {
		builderPool.Put(p)
	}
}
