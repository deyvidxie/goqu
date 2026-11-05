package sqlgen

import (
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
)

type conflictUpdateBuilder struct {
	sb.SQLBuilder
	rowAlias     exp.IdentifierExpression
	usesExcluded bool
}
