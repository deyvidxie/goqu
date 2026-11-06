package mysql

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
)

func DialectOptions() *goqu.SQLDialectOptions {
	opts := goqu.DefaultDialectOptions()

	opts.SupportsReturn = false
	opts.SupportsMySQLJSONFunctions = true
	opts.SupportsOrderByOnUpdate = true
	opts.SupportsLimitOnUpdate = true
	opts.SupportsLimitOnDelete = true
	opts.SupportsOrderByOnDelete = true
	opts.SupportsConflictUpdateWhere = false
	opts.SupportsInsertIgnoreSyntax = true
	opts.SupportsConflictTarget = false
	opts.SupportsWithCTE = false
	opts.SupportsWithCTERecursive = false
	opts.SupportsDistinctOn = false
	opts.SupportsWindowFunction = false
	opts.SupportsDeleteTableHint = true

	opts.UseFromClauseForMultipleUpdateTables = false

	opts.PlaceHolderFragment = []byte("?")
	opts.IncludePlaceholderNum = false
	opts.QuoteRune = '`'
	opts.DefaultValuesFragment = []byte("")
	opts.True = []byte("1")
	opts.False = []byte("0")
	opts.TimeFormat = "2006-01-02 15:04:05"
	opts.ForShareFragment = []byte(" LOCK IN SHARE MODE ")
	opts.UnsupportedLockStrengths = []exp.LockStrength{exp.ForNoKeyUpdate, exp.ForKeyShare}
	opts.UnsupportedLockWaitOptions = []exp.WaitOption{exp.NoWait, exp.SkipLocked}
	opts.SupportsLockOf = false
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
		exp.LikeOp:           []byte("LIKE BINARY"),
		exp.NotLikeOp:        []byte("NOT LIKE BINARY"),
		exp.ILikeOp:          []byte("LIKE"),
		exp.NotILikeOp:       []byte("NOT LIKE"),
		exp.RegexpLikeOp:     []byte("REGEXP BINARY"),
		exp.RegexpNotLikeOp:  []byte("NOT REGEXP BINARY"),
		exp.RegexpILikeOp:    []byte("REGEXP"),
		exp.RegexpNotILikeOp: []byte("NOT REGEXP"),
	}
	opts.BitwiseOperatorLookup = map[exp.BitwiseOperation][]byte{
		exp.BitwiseInversionOp:  []byte("~"),
		exp.BitwiseOrOp:         []byte("|"),
		exp.BitwiseAndOp:        []byte("&"),
		exp.BitwiseXorOp:        []byte("^"),
		exp.BitwiseLeftShiftOp:  []byte("<<"),
		exp.BitwiseRightShiftOp: []byte(">>"),
	}
	opts.FunctionNameLookup = map[string][]byte{
		exp.FunctionNameGreatest: []byte("GREATEST"),
		exp.FunctionNameLeast:    []byte("LEAST"),
		exp.FunctionNameUpper:    []byte("UCASE"),
		exp.FunctionNameLower:    []byte("LCASE"),
	}
	opts.EscapedRunes = map[rune][]byte{
		'\'': []byte("''"),
		'\\': []byte("\\\\"),
		0:    []byte("\\0"),
		0x1a: []byte("\\Z"),
	}
	opts.JoinTypeLookup[exp.StraightJoinType] = []byte(" STRAIGHT_JOIN ")
	opts.InsertIgnoreClause = []byte("INSERT IGNORE INTO")
	opts.ConflictFragment = []byte("")
	opts.ConflictDoUpdateFragment = []byte(" ON DUPLICATE KEY UPDATE ")
	opts.ConflictDoNothingFragment = []byte("")
	opts.ExcludedStyle = sqlgen.ExcludedValuesFuncStyle
	return opts
}

func DialectOptionsV8() *goqu.SQLDialectOptions {
	opts := DialectOptions()
	opts.SupportsWindowFunction = true
	// MySQL 8.0 supports WITH and WITH RECURSIVE common table expressions.
	opts.SupportsWithCTE = true
	opts.SupportsWithCTERecursive = true
	opts.ForShareFragment = []byte(" FOR SHARE ")
	opts.UnsupportedLockWaitOptions = nil
	opts.SupportsLockOf = true
	opts.ExcludedStyle = sqlgen.ExcludedRowAliasStyle
	return opts
}

func DialectOptionsV8Nano() *goqu.SQLDialectOptions {
	opts := DialectOptionsV8()
	opts.TimeFormat = "2006-01-02 15:04:05.999999999"
	return opts
}

func init() {
	goqu.RegisterDialect("mysql", "db", DialectOptions())
	goqu.RegisterDialect("mysql8", "db", DialectOptionsV8())
	goqu.RegisterDialect("mysql8nano", "db", DialectOptionsV8Nano())
}
