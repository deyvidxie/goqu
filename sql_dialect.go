package goqu

import (
	"strings"
	"sync"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlbuilder"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

type (
	SQLDialectOptions = sqlgen.SQLDialectOptions
	// An adapter interface to be used by a Dataset to generate SQL for a specific dialect.
	// See DefaultAdapter for a concrete implementation and examples.
	SQLDialect interface {
		Dialect() string
		ToSelectSQL(b sqlbuilder.SQLBuilder, clauses exp.SelectClauses)
		ToUpdateSQL(b sqlbuilder.SQLBuilder, clauses exp.UpdateClauses)
		ToInsertSQL(b sqlbuilder.SQLBuilder, clauses exp.InsertClauses)
		ToDeleteSQL(b sqlbuilder.SQLBuilder, clauses exp.DeleteClauses)
		ToTruncateSQL(b sqlbuilder.SQLBuilder, clauses exp.TruncateClauses)
	}
	// The default adapter. This class should be used when building a new adapter. When creating a new adapter you can
	// either override methods, or more typically update default values.
	// See (github.com/doug-martin/goqu/dialect/postgres)
	sqlDialect struct {
		dialect        string
		tagName        string
		dialectOptions *SQLDialectOptions
		selectGen      sqlgen.SelectSQLGenerator
		updateGen      sqlgen.UpdateSQLGenerator
		insertGen      sqlgen.InsertSQLGenerator
		deleteGen      sqlgen.DeleteSQLGenerator
		truncateGen    sqlgen.TruncateSQLGenerator
		expressionGen  sqlgen.ExpressionSQLGenerator
	}
)

type taggedDialectKey struct {
	name    string
	tagName string
}

var (
	dialects              = make(map[string]*sqlDialect)
	taggedDialects        = make(map[taggedDialectKey]*sqlDialect)
	DefaultDialectOptions = sqlgen.DefaultDialectOptions
	dialectsMu            sync.RWMutex
)

func init() {
	RegisterDialect("default", "db", DefaultDialectOptions())
}

func RegisterDialect(name, tagName string, do *SQLDialectOptions) {
	dialectsMu.Lock()
	defer dialectsMu.Unlock()
	lowerName := strings.ToLower(name)
	dialects[lowerName] = newDialect(lowerName, tagName, do)
	dropTaggedDialects(lowerName)
}

func DeregisterDialect(name string) {
	dialectsMu.Lock()
	defer dialectsMu.Unlock()
	lowerName := strings.ToLower(name)
	delete(dialects, lowerName)
	dropTaggedDialects(lowerName)
}

func dropTaggedDialects(name string) {
	for key := range taggedDialects {
		if key.name == name {
			delete(taggedDialects, key)
		}
	}
}

func registeredDialect(name string) (*sqlDialect, bool) {
	dialectsMu.RLock()
	defer dialectsMu.RUnlock()
	d, ok := dialects[strings.ToLower(name)]
	return d, ok
}

func GetDialect(name string) (SQLDialect, error) {
	if d, ok := registeredDialect(name); ok {
		return d, nil
	}
	return nil, errUnknownDialect(name)
}

type tagRequest struct {
	name     string
	explicit bool
}

func requestTag(tagName string) tagRequest {
	return tagRequest{name: tagName, explicit: tagName != ""}
}

func explicitTag(tagName string) tagRequest {
	return tagRequest{name: tagName, explicit: true}
}

func (r tagRequest) resolve(dialect string) string {
	if r.explicit {
		return r.name
	}
	return registeredTagName(dialect)
}

func registeredTagName(name string) string {
	dialectsMu.RLock()
	defer dialectsMu.RUnlock()
	if d, ok := dialects[strings.ToLower(name)]; ok {
		return d.tagName
	}
	return ""
}

func GetDialectWithTag(name, requestedTag string) (SQLDialect, error) {
	d, err := dialectFor(name, requestTag(requestedTag).resolve(name))
	if err != nil {
		return nil, err
	}
	return d, nil
}

func dialectFor(name, tagName string) (*sqlDialect, error) {
	key := taggedDialectKey{name: strings.ToLower(name), tagName: tagName}
	dialectsMu.RLock()
	d, found, err := knownDialect(name, key)
	dialectsMu.RUnlock()
	if found {
		return d, err
	}

	dialectsMu.Lock()
	defer dialectsMu.Unlock()
	d, found, err = knownDialect(name, key)
	if !found {
		d = newDialect(key.name, key.tagName, dialects[key.name].dialectOptions)
		taggedDialects[key] = d
	}
	return d, err
}

func knownDialect(name string, key taggedDialectKey) (d *sqlDialect, found bool, err error) {
	registered, ok := dialects[key.name]
	switch {
	case !ok:
		return newDialect(key.name, key.tagName, DefaultDialectOptions()), true, errUnknownDialect(name)
	case registered.tagName == key.tagName:
		return registered, true, nil
	}
	d, found = taggedDialects[key]
	return d, found, nil
}

func newDialect(dialect, tagName string, do *SQLDialectOptions) *sqlDialect {
	return &sqlDialect{
		dialect:        dialect,
		tagName:        tagName,
		dialectOptions: do,
		selectGen:      sqlgen.NewSelectSQLGenerator(dialect, tagName, do),
		updateGen:      sqlgen.NewUpdateSQLGenerator(dialect, tagName, do),
		insertGen:      sqlgen.NewInsertSQLGenerator(dialect, tagName, do),
		deleteGen:      sqlgen.NewDeleteSQLGenerator(dialect, tagName, do),
		truncateGen:    sqlgen.NewTruncateSQLGenerator(dialect, tagName, do),
		expressionGen:  sqlgen.NewExpressionSQLGenerator(dialect, tagName, do),
	}
}

func (d *sqlDialect) Dialect() string {
	return d.dialect
}

func (d *sqlDialect) ToSelectSQL(b sqlbuilder.SQLBuilder, clauses exp.SelectClauses) {
	d.selectGen.Generate(b, clauses)
}

func (d *sqlDialect) ToUpdateSQL(b sqlbuilder.SQLBuilder, clauses exp.UpdateClauses) {
	d.updateGen.Generate(b, clauses)
}

func (d *sqlDialect) ToInsertSQL(b sqlbuilder.SQLBuilder, clauses exp.InsertClauses) {
	d.insertGen.Generate(b, clauses)
}

func (d *sqlDialect) ToDeleteSQL(b sqlbuilder.SQLBuilder, clauses exp.DeleteClauses) {
	d.deleteGen.Generate(b, clauses)
}

func (d *sqlDialect) ToTruncateSQL(b sqlbuilder.SQLBuilder, clauses exp.TruncateClauses) {
	d.truncateGen.Generate(b, clauses)
}

func (d *sqlDialect) toExpressionSQL(b sqlbuilder.SQLBuilder, e exp.Expression) {
	d.expressionGen.Generate(b, e)
}
