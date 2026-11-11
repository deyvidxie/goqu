package duckdb

import (
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

func DialectOptions() *goqu.SQLDialectOptions {
	opts := goqu.DefaultDialectOptions()
	opts.TimeStyle = sqlgen.DuckDBTimeStyle

	opts.SupportsReturn = true
	opts.SupportsTruncateIdentity = false
	opts.SupportsMultipleTruncateTables = false
	opts.SupportsConflictTarget = true
	opts.WrapCompoundsInParens = false
	opts.SupportsQualify = true
	opts.SelectSQLOrder = []sqlgen.SQLFragmentType{
		sqlgen.CommonTableSQLFragment,
		sqlgen.SelectSQLFragment,
		sqlgen.FromSQLFragment,
		sqlgen.JoinSQLFragment,
		sqlgen.WhereSQLFragment,
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

	opts.PlaceHolderFragment = []byte("?")
	opts.IncludePlaceholderNum = false
	opts.TimeFormat = time.RFC3339Nano
	opts.BooleanOperatorLookup = map[exp.BooleanOperation][]byte{
		exp.EqOp:       []byte("="),
		exp.NeqOp:      []byte("!="),
		exp.GtOp:       []byte(">"),
		exp.GteOp:      []byte(">="),
		exp.LtOp:       []byte("<"),
		exp.LteOp:      []byte("<="),
		exp.InOp:       []byte("IN"),
		exp.NotInOp:    []byte("NOT IN"),
		exp.IsOp:       []byte("IS"),
		exp.IsNotOp:    []byte("IS NOT"),
		exp.LikeOp:     []byte("LIKE"),
		exp.NotLikeOp:  []byte("NOT LIKE"),
		exp.ILikeOp:    []byte("ILIKE"),
		exp.NotILikeOp: []byte("NOT ILIKE"),
	}
	opts.BitwiseOperatorLookup = map[exp.BitwiseOperation][]byte{
		exp.BitwiseInversionOp:  []byte("~"),
		exp.BitwiseOrOp:         []byte("|"),
		exp.BitwiseAndOp:        []byte("&"),
		exp.BitwiseLeftShiftOp:  []byte("<<"),
		exp.BitwiseRightShiftOp: []byte(">>"),
	}
	opts.FunctionNameLookup = map[string][]byte{
		exp.FunctionNameGreatest: []byte("GREATEST"),
		exp.FunctionNameLeast:    []byte("LEAST"),
		exp.FunctionNameUpper:    []byte("UPPER"),
		exp.FunctionNameLower:    []byte("LOWER"),
	}
	opts.EscapedRunes = map[rune][]byte{
		'\'': []byte("''"),
	}
	opts.ConflictFragment = []byte(" ON CONFLICT ")
	opts.ConflictDoUpdateFragment = []byte(" DO UPDATE SET ")
	opts.ConflictDoNothingFragment = []byte(" DO NOTHING ")
	opts.UnsupportedLockStrengths = []exp.LockStrength{exp.ForUpdate, exp.ForNoKeyUpdate, exp.ForShare, exp.ForKeyShare}

	opts.LikeEscapeKey = "ESCAPE"
	opts.LikeEscapeValue = "\\"
	return opts
}

func init() {
	goqu.RegisterDialect("duckdb", "db", DialectOptions())
}
