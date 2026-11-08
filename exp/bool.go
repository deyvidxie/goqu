package exp

import (
	"database/sql/driver"
	"reflect"
	"regexp"
	"slices"

	"github.com/doug-martin/goqu/v9/internal/util"
)

type boolean struct {
	lhs Expression
	rhs interface{}
	op  BooleanOperation
}

func NewBooleanExpression(op BooleanOperation, lhs Expression, rhs interface{}) BooleanExpression {
	return boolean{op: op, lhs: lhs, rhs: rhs}
}

func (b boolean) Clone() Expression {
	return NewBooleanExpression(b.op, b.lhs.Clone(), b.rhs)
}

func (b boolean) Expression() Expression {
	return b
}

func (b boolean) RHS() interface{} {
	return b.rhs
}

func (b boolean) LHS() Expression {
	return b.lhs
}

func (b boolean) Op() BooleanOperation {
	return b.op
}

func (b boolean) As(val interface{}) AliasedExpression {
	return NewAliasExpression(b, val)
}

// used internally to create an equality BooleanExpression
func eq(lhs Expression, rhs interface{}) BooleanExpression {
	return checkBoolExpType(EqOp, lhs, rhs, false)
}

// used internally to create an in-equality BooleanExpression
func neq(lhs Expression, rhs interface{}) BooleanExpression {
	return checkBoolExpType(EqOp, lhs, rhs, true)
}

// used internally to create an gt comparison BooleanExpression
func gt(lhs Expression, rhs interface{}) BooleanExpression {
	return NewBooleanExpression(GtOp, lhs, rhs)
}

// used internally to create an gte comparison BooleanExpression
func gte(lhs Expression, rhs interface{}) BooleanExpression {
	return NewBooleanExpression(GteOp, lhs, rhs)
}

// used internally to create an lt comparison BooleanExpression
func lt(lhs Expression, rhs interface{}) BooleanExpression {
	return NewBooleanExpression(LtOp, lhs, rhs)
}

// used internally to create an lte comparison BooleanExpression
func lte(lhs Expression, rhs interface{}) BooleanExpression {
	return NewBooleanExpression(LteOp, lhs, rhs)
}

func inNotIn(lhs Expression, op BooleanOperation, vals ...interface{}) BooleanExpression {
	if len(vals) == 1 && isNilList(vals[0]) {
		return NewBooleanExpression(op, lhs, []interface{}{})
	}
	if len(vals) == 1 {
		if util.IsListValue(vals[0]) {
			return NewBooleanExpression(op, lhs, cloneSlice(vals[0]))
		}
		if _, ok := vals[0].(AppendableExpression); ok {
			return NewBooleanExpression(op, lhs, vals[0])
		}
	}
	return NewBooleanExpression(op, lhs, slices.Clone(vals))
}

func cloneArgs(args []interface{}) []interface{} {
	if args == nil {
		return nil
	}
	cloned := make([]interface{}, len(args))
	for i, arg := range args {
		cloned[i] = cloneSlice(arg)
	}
	return cloned
}

func cloneSlice(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() && rv.Elem().Kind() == reflect.Array {
		ptr := reflect.New(rv.Elem().Type())
		ptr.Elem().Set(rv.Elem())
		return ptr.Interface()
	}
	if rv.Kind() == reflect.Ptr && !rv.IsNil() && rv.Elem().Kind() == reflect.Slice {
		ptr := reflect.New(rv.Elem().Type())
		ptr.Elem().Set(reflect.ValueOf(cloneSlice(rv.Elem().Interface())))
		return ptr.Interface()
	}
	if rv.Kind() != reflect.Slice || rv.IsNil() {
		return v
	}
	cp := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
	reflect.Copy(cp, rv)
	return cp.Interface()
}

func isNilList(v interface{}) bool {
	if v == nil {
		return true
	}
	if util.IsValuerOrBytes(v) {
		return false
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil() && util.IsListType(rv.Type().Elem())
}

func in(lhs Expression, vals ...interface{}) BooleanExpression {
	return inNotIn(lhs, InOp, vals...)
}

func notIn(lhs Expression, vals ...interface{}) BooleanExpression {
	return inNotIn(lhs, NotInOp, vals...)
}

// used internally to create an IS BooleanExpression
func is(lhs Expression, val interface{}) BooleanExpression {
	return checkBoolExpType(IsOp, lhs, val, false)
}

// used internally to create an IS NOT BooleanExpression
func isNot(lhs Expression, val interface{}) BooleanExpression {
	return checkBoolExpType(IsOp, lhs, val, true)
}

// used internally to create a LIKE BooleanExpression
func like(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(LikeOp, lhs, val, false)
}

// used internally to create an ILIKE BooleanExpression
func iLike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(ILikeOp, lhs, val, false)
}

// used internally to create a NOT LIKE BooleanExpression
func notLike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(LikeOp, lhs, val, true)
}

// used internally to create a NOT ILIKE BooleanExpression
func notILike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(ILikeOp, lhs, val, true)
}

// used internally to create a LIKE BooleanExpression
func regexpLike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(RegexpLikeOp, lhs, val, false)
}

// used internally to create an ILIKE BooleanExpression
func regexpILike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(RegexpILikeOp, lhs, val, false)
}

// used internally to create a NOT LIKE BooleanExpression
func regexpNotLike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(RegexpLikeOp, lhs, val, true)
}

// used internally to create a NOT ILIKE BooleanExpression
func regexpNotILike(lhs Expression, val interface{}) BooleanExpression {
	return checkLikeExp(RegexpILikeOp, lhs, val, true)
}

// checks an like rhs to create the proper like expression for strings or regexps
func checkLikeExp(op BooleanOperation, lhs Expression, val interface{}, invert bool) BooleanExpression {
	rhs := val

	if t, ok := val.(*regexp.Regexp); ok {
		switch op {
		case LikeOp:
			op = RegexpLikeOp
		case ILikeOp:
			op = RegexpILikeOp
		default:
			// Other operators stay as written.
		}
		rhs = t.String()
	}
	if invert {
		op = operatorInversions[op]
	}
	return NewBooleanExpression(op, lhs, rhs)
}

// checks a boolean operation normalizing the operation based on the RHS (e.g. "a" = true vs "a" IS TRUE
func checkBoolExpType(op BooleanOperation, lhs Expression, rhs interface{}, invert bool) BooleanExpression {
	if isNullValue(rhs) {
		rhs = nil
		op = IsOp
	} else {
		switch reflect.Indirect(reflect.ValueOf(rhs)).Kind() {
		case reflect.Bool:
			op = IsOp
		case reflect.Slice, reflect.Array:
			if util.IsListValue(rhs) {
				op = InOp
				rhs = cloneSlice(rhs)
			}
		case reflect.Struct:
			switch rhs.(type) {
			case SQLExpression:
				op = InOp
			case AppendableExpression:
				op = InOp
			case *regexp.Regexp:
				return checkLikeExp(LikeOp, lhs, rhs, invert)
			}
		default:
		}
	}
	if invert {
		op = operatorInversions[op]
	}
	return NewBooleanExpression(op, lhs, rhs)
}

func isNullValue(val interface{}) bool {
	if util.IsNilOrNilPointer(val) {
		return true
	}
	if util.IsBytesValue(val) {
		return reflect.Indirect(reflect.ValueOf(val)).IsNil()
	}
	if valuer, ok := val.(driver.Valuer); ok {
		v, err := valuer.Value()
		return err == nil && v == nil
	}
	return false
}
