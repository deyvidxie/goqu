package exp

type excluded struct {
	col string
}

func NewExcludedExpression(col interface{}) ExcludedExpression {
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

func (e excluded) Col() string { return e.col }

func (e excluded) Eq(val interface{}) BooleanExpression  { return eq(e, val) }
func (e excluded) Neq(val interface{}) BooleanExpression { return neq(e, val) }
func (e excluded) Gt(val interface{}) BooleanExpression  { return gt(e, val) }
func (e excluded) Gte(val interface{}) BooleanExpression { return gte(e, val) }
func (e excluded) Lt(val interface{}) BooleanExpression  { return lt(e, val) }
func (e excluded) Lte(val interface{}) BooleanExpression { return lte(e, val) }

func (e excluded) Add(val interface{}) ArithmeticExpression { return arithmeticAdd(e, val) }
func (e excluded) Sub(val interface{}) ArithmeticExpression { return arithmeticSub(e, val) }
func (e excluded) Mul(val interface{}) ArithmeticExpression { return arithmeticMul(e, val) }
func (e excluded) Div(val interface{}) ArithmeticExpression { return arithmeticDiv(e, val) }

func (e excluded) Is(val interface{}) BooleanExpression    { return is(e, val) }
func (e excluded) IsNot(val interface{}) BooleanExpression { return isNot(e, val) }
func (e excluded) IsNull() BooleanExpression               { return is(e, nil) }
func (e excluded) IsNotNull() BooleanExpression            { return isNot(e, nil) }
func (e excluded) IsTrue() BooleanExpression               { return is(e, true) }
func (e excluded) IsNotTrue() BooleanExpression            { return isNot(e, true) }
func (e excluded) IsFalse() BooleanExpression              { return is(e, false) }
func (e excluded) IsNotFalse() BooleanExpression           { return isNot(e, false) }

func (e excluded) In(vals ...interface{}) BooleanExpression    { return in(e, vals...) }
func (e excluded) NotIn(vals ...interface{}) BooleanExpression { return notIn(e, vals...) }

func (e excluded) Between(val RangeVal) RangeExpression    { return between(e, val) }
func (e excluded) NotBetween(val RangeVal) RangeExpression { return notBetween(e, val) }
