package sqlite3

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

func DialectOptions() *goqu.SQLDialectOptions {
	opts := goqu.DefaultDialectOptions()
	opts.TimeStyle = sqlgen.SQLiteTimeStyle

	opts.SupportsReturn = true
	opts.SupportsConflictUpdateWhere = true
	opts.SupportsInsertIgnoreSyntax = true
	opts.SupportsConflictWithDefaultValues = false
	opts.DefaultFragment = nil
	opts.SupportsConflictTarget = true
	opts.SupportsMultipleUpdateTables = true
	opts.WrapCompoundsInParens = false
	opts.SupportsDistinctOn = false
	opts.SupportsLateral = false

	opts.TruncateClause = []byte("DELETE FROM")
	opts.SupportsTruncateIdentity = false
	opts.SupportsTruncateCascade = false
	opts.SupportsMultipleTruncateTables = false

	opts.SupportsLimitAll = false
	opts.OffsetWithoutLimitFragment = []byte(" LIMIT -1")

	opts.PlaceHolderFragment = []byte("?")
	opts.IncludePlaceholderNum = false
	opts.QuoteRune = '`'
	opts.True = []byte("1")
	opts.False = []byte("0")
	opts.TimeFormat = "2006-01-02 15:04:05.999999999-07:00"
	opts.BooleanOperatorLookup = map[exp.BooleanOperation][]byte{
		exp.EqOp:             []byte("="),
		exp.NeqOp:            []byte("!="),
		exp.GtOp:             []byte(">"),
		exp.GteOp:            []byte(">="),
		exp.LtOp:             []byte("<"),
		exp.LteOp:            []byte("<="),
		exp.InOp:             []byte("IN"),
		exp.NotInOp:          []byte("NOT IN"),
		exp.IsOp:             []byte("IS"),
		exp.IsNotOp:          []byte("IS NOT"),
		exp.ILikeOp:          []byte("LIKE"),
		exp.NotILikeOp:       []byte("NOT LIKE"),
		exp.RegexpLikeOp:     []byte("REGEXP"),
		exp.RegexpNotLikeOp:  []byte("NOT REGEXP"),
		exp.RegexpILikeOp:    []byte("REGEXP '(?i)' ||"),
		exp.RegexpNotILikeOp: []byte("NOT REGEXP '(?i)' ||"),
	}
	opts.UseLiteralIsBools = false
	opts.BitwiseOperatorLookup = map[exp.BitwiseOperation][]byte{
		exp.BitwiseOrOp:         []byte("|"),
		exp.BitwiseAndOp:        []byte("&"),
		exp.BitwiseLeftShiftOp:  []byte("<<"),
		exp.BitwiseRightShiftOp: []byte(">>"),
	}
	opts.FunctionNameLookup = map[string][]byte{
		exp.FunctionNameGreatest: []byte("MAX"),
		exp.FunctionNameLeast:    []byte("MIN"),
		exp.FunctionNameUpper:    []byte("UPPER"),
		exp.FunctionNameLower:    []byte("LOWER"),
	}
	opts.EscapedRunes = map[rune][]byte{
		'\'': []byte("''"),
	}
	opts.InsertIgnoreClause = []byte("INSERT OR IGNORE INTO ")
	opts.ConflictFragment = []byte(" ON CONFLICT ")
	opts.ConflictDoUpdateFragment = []byte(" DO UPDATE SET ")
	opts.ConflictDoNothingFragment = []byte(" DO NOTHING ")
	opts.UnsupportedLockStrengths = []exp.LockStrength{exp.ForUpdate, exp.ForNoKeyUpdate, exp.ForShare, exp.ForKeyShare}

	opts.LikeEscapeKey = "ESCAPE"
	opts.LikeEscapeValue = "\\"
	return opts
}

func init() {
	goqu.RegisterDialect("sqlite3", "db", DialectOptions())
}
