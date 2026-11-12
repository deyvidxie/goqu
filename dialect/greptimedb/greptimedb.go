package greptimedb

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

func DialectOptions() *goqu.SQLDialectOptions {
	opts := goqu.DefaultDialectOptions()
	opts.TimeStyle = sqlgen.GreptimeDBTimeStyle

	opts.SupportsQualify = true
	opts.SupportsReturn = false
	opts.SupportsLateral = false
	opts.SupportsOnConflict = false
	opts.SupportsMultipleTruncateTables = false
	opts.SupportsTruncateCascade = false
	opts.SupportsTruncateIdentity = false
	opts.UnsupportedLockStrengths = []exp.LockStrength{exp.ForUpdate, exp.ForNoKeyUpdate, exp.ForShare, exp.ForKeyShare}
	opts.UpdateSQLOrder = nil

	opts.TruncateClause = []byte("TRUNCATE TABLE")
	opts.QuoteRune = '`'
	opts.EscapedRunes = map[rune][]byte{
		'\'': []byte("''"),
		'\\': []byte("\\\\"),
		0:    []byte("\\0"),
		0x1a: []byte("\\Z"),
	}
	delete(opts.BitwiseOperatorLookup, exp.BitwiseInversionOp)
	opts.BitwiseOperatorLookup[exp.BitwiseXorOp] = []byte("^")
	return opts
}

func init() {
	goqu.RegisterDialect("greptimedb", "db", DialectOptions())
}
