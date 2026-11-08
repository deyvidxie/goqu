package sqlgen

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestInvalidExpressionErrors(t *testing.T) {
	join := exp.NewConditionedJoinExpression(exp.LeftJoinType, exp.NewIdentifierExpression("", "t", ""), nil)
	for _, c := range []struct {
		name string
		want string
		err  error
	}{
		{
			"errEmptyIdentifier", `goqu: an empty identifier was encountered, please specify a "schema", "table" or "column"`,
			errEmptyIdentifier,
		},
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
		{"errNoSourceForTruncate", "goqu: no source found when generating truncate sql", errNoSourceForTruncate},
		{"errExcludedOutsideConflictUpdate", "goqu: Excluded is only valid in an OnConflict DoUpdate", errExcludedOutsideConflictUpdate},
		{"errExcludedWithoutColumn", "goqu: Excluded requires a column name", errExcludedWithoutColumn},
		{"errExcludedRowAliasQualified", "goqu: Excluded requires an unqualified insert alias", errExcludedRowAliasQualified},
		{
			"errExcludedRowAliasUnresolved",
			"goqu: Excluded rendered differently on a second pass; Valuers and expressions in DoUpdate must render deterministically",
			errExcludedRowAliasUnresolved,
		},
		{"errIntervalCountCondition", "goqu: Interval count goqu.Ex/ExOr is a condition, not a count", errIntervalCountCondition},
		{"errJSONCandidateInvalid", "goqu: JSONContains candidate is not valid JSON", errJSONCandidateInvalid},
		{
			"errJSONCandidateCondition", "goqu: JSONContains candidate goqu.Ex/ExOr is a condition, not a JSON object; use map[string]any",
			errJSONCandidateCondition,
		},
		{"errEmptyInsertAlias", "goqu: Excluded requires a non-empty insert alias", errEmptyInsertAlias},
	} {
		assert.EqualError(t, c.err, c.want, c.name)
		assert.True(t, errors.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}
}
