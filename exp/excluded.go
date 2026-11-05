package exp

type excluded struct {
	col string
}

func NewExcludedExpression(col any) ExcludedExpression {
	var ident IdentifierExpression
	switch c := col.(type) {
	case string:
		ident = ParseIdentifier(c)
	case IdentifierExpression:
		ident = c
	default:
		return excluded{}
	}
	name, _ := ident.GetCol().(string)
	return excluded{col: name}
}

func (e excluded) Expression() Expression { return e }

func (e excluded) Clone() Expression { return e }

func (e excluded) GetCol() string { return e.col }

func (e excluded) Eq(val any) BooleanExpression  { return eq(e, val) }
func (e excluded) Neq(val any) BooleanExpression { return neq(e, val) }
func (e excluded) Gt(val any) BooleanExpression  { return gt(e, val) }
func (e excluded) Gte(val any) BooleanExpression { return gte(e, val) }
func (e excluded) Lt(val any) BooleanExpression  { return lt(e, val) }
func (e excluded) Lte(val any) BooleanExpression { return lte(e, val) }

func (e excluded) Add(val any) ArithmeticExpression { return arithmeticAdd(e, val) }
func (e excluded) Sub(val any) ArithmeticExpression { return arithmeticSub(e, val) }
func (e excluded) Mul(val any) ArithmeticExpression { return arithmeticMul(e, val) }
func (e excluded) Div(val any) ArithmeticExpression { return arithmeticDiv(e, val) }

func (e excluded) Is(val any) BooleanExpression    { return is(e, val) }
func (e excluded) IsNot(val any) BooleanExpression { return isNot(e, val) }
func (e excluded) IsNull() BooleanExpression       { return is(e, nil) }
func (e excluded) IsNotNull() BooleanExpression    { return isNot(e, nil) }
func (e excluded) IsTrue() BooleanExpression       { return is(e, true) }
func (e excluded) IsNotTrue() BooleanExpression    { return isNot(e, true) }
func (e excluded) IsFalse() BooleanExpression      { return is(e, false) }
func (e excluded) IsNotFalse() BooleanExpression   { return isNot(e, false) }

func (e excluded) In(vals ...any) BooleanExpression    { return in(e, vals...) }
func (e excluded) NotIn(vals ...any) BooleanExpression { return notIn(e, vals...) }

func (e excluded) Between(val RangeVal) RangeExpression    { return between(e, val) }
func (e excluded) NotBetween(val RangeVal) RangeExpression { return notBetween(e, val) }

func (e excluded) Like(val any) BooleanExpression           { return like(e, val) }
func (e excluded) NotLike(val any) BooleanExpression        { return notLike(e, val) }
func (e excluded) ILike(val any) BooleanExpression          { return iLike(e, val) }
func (e excluded) NotILike(val any) BooleanExpression       { return notILike(e, val) }
func (e excluded) RegexpLike(val any) BooleanExpression     { return regexpLike(e, val) }
func (e excluded) RegexpNotLike(val any) BooleanExpression  { return regexpNotLike(e, val) }
func (e excluded) RegexpILike(val any) BooleanExpression    { return regexpILike(e, val) }
func (e excluded) RegexpNotILike(val any) BooleanExpression { return regexpNotILike(e, val) }

func (e excluded) BitwiseInversion() BitwiseExpression  { return bitwiseInversion(e) }
func (e excluded) BitwiseOr(val any) BitwiseExpression  { return bitwiseOr(e, val) }
func (e excluded) BitwiseAnd(val any) BitwiseExpression { return bitwiseAnd(e, val) }
func (e excluded) BitwiseXor(val any) BitwiseExpression { return bitwiseXor(e, val) }
func (e excluded) BitwiseLeftShift(val any) BitwiseExpression {
	return bitwiseLeftShift(e, val)
}
func (e excluded) BitwiseRightShift(val any) BitwiseExpression {
	return bitwiseRightShift(e, val)
}
