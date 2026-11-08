package sqlgen

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/stretchr/testify/assert"
)

func TestInvalidExpressionValues(t *testing.T) {
	join := exp.NewConditionedJoinExpression(exp.LeftJoinType, exp.NewIdentifierExpression("", "t", ""), nil)
	for _, c := range []struct {
		name string
		want string
		err  error
	}{
		{"errEmptyIdentifier", "an empty identifier was encountered", errEmptyIdentifier},
		{"errUnexpectedNamedWindow", "unexpected named window function", errUnexpectedNamedWindow},
		{"errEmptyCaseWhens", "when conditions not found for case statement", errEmptyCaseWhens},
		{"errNoWindowName", "window expression has no valid name", errNoWindowName},
		{"errInvalidCollation", `invalid collation name "x"`, errInvalidCollation("x")},
		{"errJoinConditionRequired", "join condition required for conditioned join LeftJoinType", errJoinConditionRequired(join)},
		{"errConflictUpdateValuesRequired", "values are required for on conflict update", errConflictUpdateValuesRequired},
		{"errNoUpdatedValuesProvided", "no update values provided", errNoUpdatedValuesProvided},
		{"errNoSourceForDelete", "no source found when generating delete sql", errNoSourceForDelete},
		{"errNoSourceForInsert", "no source found when generating insert sql", errNoSourceForInsert},
		{"errNoSourceForUpdate", "no source found when generating update sql", errNoSourceForUpdate},
		{"errNoSetValuesForUpdate", "no set values found when generating update sql", errNoSetValuesForUpdate},
	} {
		assert.ErrorContains(t, c.err, c.want, c.name)
		assert.True(t, errors.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}
}

func TestSentinelErrorsKeepIdentityAndMessage(t *testing.T) {
	opts := DefaultDialectOptions()
	for _, c := range []struct {
		sentinel error
		want     string
		generate func(sb.SQLBuilder)
	}{
		{errNoSourceForDelete, "goqu: no source found when generating delete sql", func(b sb.SQLBuilder) {
			NewDeleteSQLGenerator("test", "db", opts).Generate(b, exp.NewDeleteClauses())
		}},
		{errNoSourceForInsert, "goqu: no source found when generating insert sql", func(b sb.SQLBuilder) {
			NewInsertSQLGenerator("test", "db", opts).Generate(b, exp.NewInsertClauses())
		}},
		{errNoSourceForUpdate, "goqu: no source found when generating update sql", func(b sb.SQLBuilder) {
			NewUpdateSQLGenerator("test", "db", opts).Generate(b, exp.NewUpdateClauses())
		}},
		{errNoSetValuesForUpdate, "goqu: no set values found when generating update sql", func(b sb.SQLBuilder) {
			uc := exp.NewUpdateClauses().SetTable(exp.NewIdentifierExpression("", "t", ""))
			NewUpdateSQLGenerator("test", "db", opts).Generate(b, uc)
		}},
	} {
		b := sb.NewSQLBuilder(false)
		c.generate(b)
		_, _, err := b.ToSQL()
		assert.ErrorIs(t, err, c.sentinel)
		assert.EqualError(t, err, c.want)
	}
}
