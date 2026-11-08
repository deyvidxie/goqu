package sqlgen

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestInvalidExpressionValues(t *testing.T) {
	join := exp.NewConditionedJoinExpression(exp.LeftJoinType, exp.NewIdentifierExpression("", "t", ""), nil)
	for _, c := range []struct {
		name string
		want string
		err  error
	}{
		{"errEmptyIdentifier", `goqu: an empty identifier was encountered, please specify a "schema", "table" or "column"`, errEmptyIdentifier},
		{"errUnexpectedNamedWindow", "goqu: unexpected named window function", errUnexpectedNamedWindow},
		{"errEmptyCaseWhens", "goqu: when conditions not found for case statement", errEmptyCaseWhens},
		{"errNoWindowName", "goqu: window expression has no valid name", errNoWindowName},
		{"errInvalidCollation", `goqu: invalid collation name "x"`, errInvalidCollation("x")},
		{"errJoinConditionRequired", "goqu: join condition required for conditioned join LeftJoinType", errJoinConditionRequired(join)},
		{"errConflictUpdateValuesRequired", `goqu: values are required for on conflict update expression`, errConflictUpdateValuesRequired},
		{"errNoUpdatedValuesProvided", "goqu: no update values provided", errNoUpdatedValuesProvided},
		{"errNoSourceForDelete", "goqu: no source found when generating delete sql", errNoSourceForDelete},
		{"errNoSourceForInsert", "goqu: no source found when generating insert sql", errNoSourceForInsert},
		{"errNoSourceForUpdate", "goqu: no source found when generating update sql", errNoSourceForUpdate},
		{"errNoSetValuesForUpdate", "goqu: no set values found when generating update sql", errNoSetValuesForUpdate},
	} {
		assert.EqualError(t, c.err, c.want, c.name)
		assert.True(t, errors.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}
}
