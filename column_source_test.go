package goqu_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
)

type (
	sourceNode struct {
		ID     int64 `db:"id"`
		Parent *sourceNode
	}
	SourceSelfEmbed struct {
		ID int64 `db:"id"`
		*SourceSelfEmbed
	}
	sourceOwner struct {
		ID   int64 `db:"id"`
		Pets []sourcePet
		Pet  *sourcePet
	}
	sourcePet struct {
		Name  string `db:"name"`
		Owner *sourceOwner
	}
	SourceCount      int
	sourceCountEmbed struct {
		ID int64 `db:"id"`
		*SourceCount
	}
	sourceRec struct {
		A int `db:"a"`
	}
)

func TestStructColumnSourcesThatCannotBuildColumnsFail(t *testing.T) {
	toSQL := func(ds interface{ ToSQL() (string, []any, error) }) error {
		_, _, err := ds.ToSQL()
		return err
	}
	const (
		derive = "goqu: cannot derive columns from goqu_test."
		tagIt  = "; tag it `db:\"-\"`"
		node   = derive + "sourceNode: field Parent leads back to goqu_test.sourceNode" + tagIt
		embed  = derive + "SourceSelfEmbed: field SourceSelfEmbed leads back to goqu_test.SourceSelfEmbed" + tagIt
		mutual = derive + "sourceOwner: field Pet.Owner leads back to goqu_test.sourceOwner" + tagIt
		count  = derive + "sourceCountEmbed: embedded field SourceCount is a pointer to non-struct goqu_test.SourceCount" + tagIt
	)
	for name, c := range map[string]struct {
		want string
		err  error
	}{
		"select":           {node, toSQL(goqu.From("t").Select(&sourceNode{}))},
		"select append":    {node, toSQL(goqu.From("t").SelectAppend(sourceNode{}))},
		"insert returning": {node, toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}).Returning(sourceNode{}))},
		"update returning": {node, toSQL(goqu.Update("t").Set(goqu.Record{"a": 1}).Returning(sourceNode{}))},
		"delete returning": {node, toSQL(goqu.Delete("t").Returning(sourceNode{}))},
		"insert rows":      {node, toSQL(goqu.Insert("t").Rows(sourceNode{ID: 1}))},
		"insert cols":      {node, toSQL(goqu.Insert("t").Cols(sourceNode{}).Vals([]interface{}{1}))},
		"update set":       {node, toSQL(goqu.Update("t").Set(sourceNode{ID: 1}))},
		"self embed":       {embed, toSQL(goqu.From("t").Select(SourceSelfEmbed{}))},
		"mutual recursion": {mutual, toSQL(goqu.Insert("t").Rows(sourceOwner{ID: 1}))},
		"non-struct embed": {count, toSQL(goqu.From("t").Select(sourceCountEmbed{}))},
		"non-struct row":   {count, toSQL(goqu.Insert("t").Rows(sourceCountEmbed{ID: 1}))},
	} {
		assert.EqualError(t, c.err, c.want, name)
		assert.True(t, goqu.IsInvalidExpression(c.err), name)
	}
}

func TestStructColumnSourcesAcceptNilPointersAndArrays(t *testing.T) {
	for _, ds := range []*goqu.SelectDataset{
		goqu.From("t").Select((*sourceRec)(nil)),
		goqu.From("t").Select((*[]sourceRec)(nil)),
		goqu.From("t").Select([2]sourceRec{}),
		goqu.From("t").Select(&[2]sourceRec{}),
		goqu.From("t").Select([]*sourceRec{}),
	} {
		sql, _, err := ds.ToSQL()
		assert.NoError(t, err)
		assert.Equal(t, `SELECT "a" FROM "t"`, sql)
	}

	sql, _, err := goqu.Insert("t").Rows(goqu.Record{"a": 1}).Returning((*sourceRec)(nil)).ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `INSERT INTO "t" ("a") VALUES (1) RETURNING "a"`, sql)
}
