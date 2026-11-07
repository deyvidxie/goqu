package exp

import "slices"

type jsonExpression struct {
	function  JSONFunction
	col       any
	candidate any
	paths     []string
}

func NewJSONExtractExpression(col any, path string) JSONExpression {
	return jsonExpression{function: JSONExtractFunction, col: jsonCol(col), paths: []string{path}}
}

func NewJSONValueExpression(col any, path string) JSONExpression {
	return jsonExpression{function: JSONValueFunction, col: jsonCol(col), paths: []string{path}}
}

func NewJSONContainsExpression(col, candidate any, paths ...string) JSONExpression {
	return jsonExpression{function: JSONContainsFunction, col: jsonCol(col), candidate: candidate, paths: slices.Clone(paths)}
}

func jsonCol(col any) any {
	if s, ok := col.(string); ok {
		return ParseIdentifier(s)
	}
	return col
}

func (je jsonExpression) Clone() Expression {
	je.paths = slices.Clone(je.paths)
	return je
}
func (je jsonExpression) Expression() Expression       { return je }
func (je jsonExpression) Function() JSONFunction       { return je.function }
func (je jsonExpression) Col() any                     { return je.col }
func (je jsonExpression) Candidate() any               { return je.candidate }
func (je jsonExpression) Paths() []string              { return je.paths }
func (je jsonExpression) As(val any) AliasedExpression { return NewAliasExpression(je, val) }

func (je jsonExpression) Eq(val any) BooleanExpression  { return eq(je, val) }
func (je jsonExpression) Neq(val any) BooleanExpression { return neq(je, val) }
func (je jsonExpression) Gt(val any) BooleanExpression  { return gt(je, val) }
func (je jsonExpression) Gte(val any) BooleanExpression { return gte(je, val) }
func (je jsonExpression) Lt(val any) BooleanExpression  { return lt(je, val) }
func (je jsonExpression) Lte(val any) BooleanExpression { return lte(je, val) }

func (je jsonExpression) Between(val RangeVal) RangeExpression    { return between(je, val) }
func (je jsonExpression) NotBetween(val RangeVal) RangeExpression { return notBetween(je, val) }

func (je jsonExpression) In(vals ...any) BooleanExpression    { return in(je, vals...) }
func (je jsonExpression) NotIn(vals ...any) BooleanExpression { return notIn(je, vals...) }

func (je jsonExpression) Is(val any) BooleanExpression    { return is(je, val) }
func (je jsonExpression) IsNot(val any) BooleanExpression { return isNot(je, val) }
func (je jsonExpression) IsNull() BooleanExpression       { return is(je, nil) }
func (je jsonExpression) IsNotNull() BooleanExpression    { return isNot(je, nil) }
func (je jsonExpression) IsTrue() BooleanExpression       { return is(je, true) }
func (je jsonExpression) IsNotTrue() BooleanExpression    { return isNot(je, true) }
func (je jsonExpression) IsFalse() BooleanExpression      { return is(je, false) }
func (je jsonExpression) IsNotFalse() BooleanExpression   { return isNot(je, false) }

func (je jsonExpression) Like(val any) BooleanExpression       { return like(je, val) }
func (je jsonExpression) NotLike(val any) BooleanExpression    { return notLike(je, val) }
func (je jsonExpression) ILike(val any) BooleanExpression      { return iLike(je, val) }
func (je jsonExpression) NotILike(val any) BooleanExpression   { return notILike(je, val) }
func (je jsonExpression) RegexpLike(val any) BooleanExpression { return regexpLike(je, val) }
func (je jsonExpression) RegexpNotLike(val any) BooleanExpression {
	return regexpNotLike(je, val)
}
func (je jsonExpression) RegexpILike(val any) BooleanExpression { return regexpILike(je, val) }
func (je jsonExpression) RegexpNotILike(val any) BooleanExpression {
	return regexpNotILike(je, val)
}

func (je jsonExpression) Add(val any) ArithmeticExpression { return arithmeticAdd(je, val) }
func (je jsonExpression) Sub(val any) ArithmeticExpression { return arithmeticSub(je, val) }
func (je jsonExpression) Mul(val any) ArithmeticExpression { return arithmeticMul(je, val) }
func (je jsonExpression) Div(val any) ArithmeticExpression { return arithmeticDiv(je, val) }

func (je jsonExpression) Asc() OrderedExpression  { return asc(je) }
func (je jsonExpression) Desc() OrderedExpression { return desc(je) }
