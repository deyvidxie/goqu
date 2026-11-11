package goqu

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exec"
	"github.com/doug-martin/goqu/v9/sqlbuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type builderCapturingQueryFactory struct {
	builder   sqlbuilder.SQLBuilder
	sqlAtCall string
}

func (f *builderCapturingQueryFactory) FromSQL(string, ...any) exec.QueryExecutor {
	return exec.QueryExecutor{}
}

func (f *builderCapturingQueryFactory) FromSQLBuilder(b sqlbuilder.SQLBuilder) exec.QueryExecutor {
	f.builder = b
	f.sqlAtCall, _, _ = b.ToSQL()
	return exec.QueryExecutor{}
}

func TestExecutorReleasesItsSQLBuilder(t *testing.T) {
	for name, executor := range map[string]func(exec.QueryFactory){
		"select": func(qf exec.QueryFactory) { newDataset("default", tagRequest{}, qf).From("t").Executor() },
		"insert": func(qf exec.QueryFactory) {
			newInsertDataset("default", tagRequest{}, qf).Into("t").Rows(Record{"a": 1}).Executor()
		},
		"update": func(qf exec.QueryFactory) {
			newUpdateDataset("default", tagRequest{}, qf).Table("t").Set(Record{"a": 1}).Executor()
		},
		"delete":   func(qf exec.QueryFactory) { newDeleteDataset("default", tagRequest{}, qf).From("t").Executor() },
		"truncate": func(qf exec.QueryFactory) { newTruncateDataset("default", tagRequest{}, qf).Table("t").Executor() },
	} {
		t.Run(name, func(t *testing.T) {
			qf := &builderCapturingQueryFactory{}
			executor(qf)
			require.NotNil(t, qf.builder)
			require.NotEmpty(t, qf.sqlAtCall)

			sqlAfter, _, err := qf.builder.ToSQL()
			assert.NoError(t, err)
			assert.Empty(t, sqlAfter)
		})
	}
}
