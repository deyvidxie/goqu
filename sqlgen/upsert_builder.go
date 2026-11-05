package sqlgen

import (
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
)

// upsertBuilder marks SQL written for ON CONFLICT DO UPDATE, the only place Excluded may appear.
type upsertBuilder struct {
	sb.SQLBuilder
	rowAlias     exp.IdentifierExpression
	usesExcluded bool
}

var (
	excludedTableFragment = []byte("EXCLUDED.")
	valuesFuncFragment    = []byte("VALUES(")

	errExcludedOutsideConflictUpdate = NewInvalidExpressionError("Excluded is only valid in an OnConflict DoUpdate")
	errExcludedWithoutColumn         = NewInvalidExpressionError("Excluded requires a column name")
)
