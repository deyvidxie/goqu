package greptimedb

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

func DialectOptions() *goqu.SQLDialectOptions {
	opts := commonOptions()
	opts.QuoteRune = '`'
	opts.EscapedRunes = map[rune][]byte{
		'\'': []byte("''"),
		'\\': []byte("\\\\"),
		0:    []byte("\\0"),
		0x1a: []byte("\\Z"),
	}
	opts.BitwiseOperatorLookup[exp.BitwiseXorOp] = []byte("^")
	return opts
}

func DialectOptionsPG() *goqu.SQLDialectOptions {
	opts := commonOptions()
	opts.PlaceHolderFragment = []byte("$")
	opts.IncludePlaceholderNum = true
	opts.QuoteCollation = true
	return opts
}

func commonOptions() *goqu.SQLDialectOptions {
	opts := goqu.DefaultDialectOptions()
	opts.TimeStyle = sqlgen.GreptimeDBTimeStyle

	opts.SupportsQualify = true
	opts.SupportsRangeQuery = true
	opts.SupportsReturn = false
	opts.SupportsLateral = false
	opts.SupportsOnConflict = false
	opts.SupportsMultipleTruncateTables = false
	opts.SupportsTruncateCascade = false
	opts.SupportsTruncateIdentity = false
	opts.UnsupportedLockStrengths = []exp.LockStrength{exp.ForUpdate, exp.ForNoKeyUpdate, exp.ForShare, exp.ForKeyShare}
	opts.UpdateSQLOrder = nil
	opts.SelectSQLOrder = []sqlgen.SQLFragmentType{
		sqlgen.CommonTableSQLFragment,
		sqlgen.SelectSQLFragment,
		sqlgen.FromSQLFragment,
		sqlgen.JoinSQLFragment,
		sqlgen.WhereSQLFragment,
		sqlgen.AlignSQLFragment,
		sqlgen.GroupBySQLFragment,
		sqlgen.HavingSQLFragment,
		sqlgen.WindowSQLFragment,
		sqlgen.QualifySQLFragment,
		sqlgen.CompoundsSQLFragment,
		sqlgen.OrderSQLFragment,
		sqlgen.LimitSQLFragment,
		sqlgen.OffsetSQLFragment,
		sqlgen.ForSQLFragment,
	}

	opts.TruncateClause = []byte("TRUNCATE TABLE")
	delete(opts.BitwiseOperatorLookup, exp.BitwiseInversionOp)
	return opts
}

func init() {
	goqu.RegisterDialect("greptimedb", "db", DialectOptions())
	goqu.RegisterDialect("greptimedb-pg", "db", DialectOptionsPG())
}
