package goqu

import (
	"strings"
	"sync"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

type (
	SQLDialectOptions = sqlgen.SQLDialectOptions
	// An adapter interface to be used by a Dataset to generate SQL for a specific dialect.
	// See DefaultAdapter for a concrete implementation and examples.
	SQLDialect interface {
		Dialect() string
		ToSelectSQL(b sb.SQLBuilder, clauses exp.SelectClauses)
		ToUpdateSQL(b sb.SQLBuilder, clauses exp.UpdateClauses)
		ToInsertSQL(b sb.SQLBuilder, clauses exp.InsertClauses)
		ToDeleteSQL(b sb.SQLBuilder, clauses exp.DeleteClauses)
		ToTruncateSQL(b sb.SQLBuilder, clauses exp.TruncateClauses)
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

func GetDialect(name string) SQLDialect {
	dialectsMu.RLock()
	defer dialectsMu.RUnlock()
	if d, ok := dialects[strings.ToLower(name)]; ok {
		return d
	}
	return newDialect("default", "db", DefaultDialectOptions())
}

func resolveTagName(dialect, tagName string) (resolved string, fromRegistry bool) {
	if tagName == "" {
		return registeredTagName(dialect), true
	}
	return tagName, false
}

func registeredTagName(name string) string {
	dialectsMu.RLock()
	defer dialectsMu.RUnlock()
	if d, ok := dialects[strings.ToLower(name)]; ok {
		return d.tagName
	}
	return "db"
}

func GetDialectWithTag(name, tagName string) SQLDialect {
	name = strings.ToLower(name)
	key := taggedDialectKey{name: name, tagName: tagName}
	dialectsMu.RLock()
	d, registered := dialects[name]
	tagged, cached := taggedDialects[key]
	dialectsMu.RUnlock()
	switch {
	case !registered:
		return newDialect("default", tagName, DefaultDialectOptions())
	case d.tagName == tagName:
		return d
	case cached:
		return tagged
	}

	dialectsMu.Lock()
	defer dialectsMu.Unlock()
	d, registered = dialects[name]
	switch {
	case !registered:
		return newDialect("default", tagName, DefaultDialectOptions())
	case d.tagName == tagName:
		return d
	}
	if tagged, cached = taggedDialects[key]; !cached {
		tagged = newDialect(name, tagName, d.dialectOptions)
		taggedDialects[key] = tagged
	}
	return tagged
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
	}
}

func (d *sqlDialect) Dialect() string {
	return d.dialect
}

func (d *sqlDialect) ToSelectSQL(b sb.SQLBuilder, clauses exp.SelectClauses) {
	d.selectGen.Generate(b, clauses)
}

func (d *sqlDialect) ToUpdateSQL(b sb.SQLBuilder, clauses exp.UpdateClauses) {
	d.updateGen.Generate(b, clauses)
}

func (d *sqlDialect) ToInsertSQL(b sb.SQLBuilder, clauses exp.InsertClauses) {
	d.insertGen.Generate(b, clauses)
}

func (d *sqlDialect) ToDeleteSQL(b sb.SQLBuilder, clauses exp.DeleteClauses) {
	d.deleteGen.Generate(b, clauses)
}

func (d *sqlDialect) ToTruncateSQL(b sb.SQLBuilder, clauses exp.TruncateClauses) {
	d.truncateGen.Generate(b, clauses)
}
