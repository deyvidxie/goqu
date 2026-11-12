package exp

const (
	AnyFunctionName = "ANY "
	AllFunctionName = "ALL "
)

type (
	sqlFunctionExpression struct {
		name        string
		args        []interface{}
		orderBy     ColumnListExpression
		withinGroup ColumnListExpression
	}
)

func NewSQLFunctionExpression(name string, args ...interface{}) SQLFunctionExpression {
	return sqlFunctionExpression{name: name, args: cloneArgs(args)}
}

func (sfe sqlFunctionExpression) Clone() Expression {
	return sqlFunctionExpression{
		name:        sfe.name,
		args:        cloneArgs(sfe.args),
		orderBy:     cloneColumnList(sfe.orderBy),
		withinGroup: cloneColumnList(sfe.withinGroup),
	}
}

func (sfe sqlFunctionExpression) Expression() Expression { return sfe }

func (sfe sqlFunctionExpression) Args() []interface{} { return sfe.args }

func (sfe sqlFunctionExpression) Name() string { return sfe.name }

func (sfe sqlFunctionExpression) OrderCols() ColumnListExpression { return sfe.orderBy }

func (sfe sqlFunctionExpression) HasOrder() bool { return sfe.orderBy != nil }

func (sfe sqlFunctionExpression) OrderBy(order ...OrderedExpression) SQLFunctionExpression {
	sfe.orderBy = orderedList("OrderBy", order)
	return sfe
}

func (sfe sqlFunctionExpression) WithinGroupCols() ColumnListExpression { return sfe.withinGroup }

func (sfe sqlFunctionExpression) HasWithinGroup() bool { return sfe.withinGroup != nil }

func (sfe sqlFunctionExpression) WithinGroup(order ...OrderedExpression) SQLFunctionExpression {
	sfe.withinGroup = orderedList("WithinGroup", order)
	return sfe
}

func orderedList(method string, order []OrderedExpression) ColumnListExpression {
	cols := NewOrderedColumnList(order...)
	if cols.IsEmpty() {
		return emptyAppendList(nil, method, "expression")
	}
	return cols
}

func (sfe sqlFunctionExpression) As(val interface{}) AliasedExpression {
	return NewAliasExpression(sfe, val)
}

func (sfe sqlFunctionExpression) Eq(val interface{}) BooleanExpression  { return eq(sfe, val) }
func (sfe sqlFunctionExpression) Neq(val interface{}) BooleanExpression { return neq(sfe, val) }

func (sfe sqlFunctionExpression) Gt(val interface{}) BooleanExpression  { return gt(sfe, val) }
func (sfe sqlFunctionExpression) Gte(val interface{}) BooleanExpression { return gte(sfe, val) }
func (sfe sqlFunctionExpression) Lt(val interface{}) BooleanExpression  { return lt(sfe, val) }
func (sfe sqlFunctionExpression) Lte(val interface{}) BooleanExpression { return lte(sfe, val) }

func (sfe sqlFunctionExpression) Between(val RangeVal) RangeExpression { return between(sfe, val) }

func (sfe sqlFunctionExpression) NotBetween(val RangeVal) RangeExpression {
	return notBetween(sfe, val)
}

func (sfe sqlFunctionExpression) Like(val interface{}) BooleanExpression    { return like(sfe, val) }
func (sfe sqlFunctionExpression) NotLike(val interface{}) BooleanExpression { return notLike(sfe, val) }
func (sfe sqlFunctionExpression) ILike(val interface{}) BooleanExpression   { return iLike(sfe, val) }

func (sfe sqlFunctionExpression) NotILike(val interface{}) BooleanExpression {
	return notILike(sfe, val)
}

func (sfe sqlFunctionExpression) RegexpLike(val interface{}) BooleanExpression {
	return regexpLike(sfe, val)
}

func (sfe sqlFunctionExpression) RegexpNotLike(val interface{}) BooleanExpression {
	return regexpNotLike(sfe, val)
}

func (sfe sqlFunctionExpression) RegexpILike(val interface{}) BooleanExpression {
	return regexpILike(sfe, val)
}

func (sfe sqlFunctionExpression) RegexpNotILike(val interface{}) BooleanExpression {
	return regexpNotILike(sfe, val)
}

func (sfe sqlFunctionExpression) In(vals ...interface{}) BooleanExpression { return in(sfe, vals...) }
func (sfe sqlFunctionExpression) NotIn(vals ...interface{}) BooleanExpression {
	return notIn(sfe, vals...)
}
func (sfe sqlFunctionExpression) Is(val interface{}) BooleanExpression    { return is(sfe, val) }
func (sfe sqlFunctionExpression) IsNot(val interface{}) BooleanExpression { return isNot(sfe, val) }
func (sfe sqlFunctionExpression) IsNull() BooleanExpression               { return is(sfe, nil) }
func (sfe sqlFunctionExpression) IsNotNull() BooleanExpression            { return isNot(sfe, nil) }
func (sfe sqlFunctionExpression) IsTrue() BooleanExpression               { return is(sfe, true) }
func (sfe sqlFunctionExpression) IsNotTrue() BooleanExpression            { return isNot(sfe, true) }
func (sfe sqlFunctionExpression) IsFalse() BooleanExpression              { return is(sfe, false) }
func (sfe sqlFunctionExpression) IsNotFalse() BooleanExpression           { return isNot(sfe, false) }

func (sfe sqlFunctionExpression) Over(we WindowExpression) SQLWindowFunctionExpression {
	return NewSQLWindowFunctionExpression(sfe, nil, we)
}

func (sfe sqlFunctionExpression) OverName(windowName IdentifierExpression) SQLWindowFunctionExpression {
	return NewSQLWindowFunctionExpression(sfe, windowName, nil)
}

func (sfe sqlFunctionExpression) Asc() OrderedExpression  { return asc(sfe) }
func (sfe sqlFunctionExpression) Desc() OrderedExpression { return desc(sfe) }

func (sfe sqlFunctionExpression) Add(val interface{}) ArithmeticExpression {
	return arithmeticAdd(sfe, val)
}
func (sfe sqlFunctionExpression) Sub(val interface{}) ArithmeticExpression {
	return arithmeticSub(sfe, val)
}
func (sfe sqlFunctionExpression) Mul(val interface{}) ArithmeticExpression {
	return arithmeticMul(sfe, val)
}
func (sfe sqlFunctionExpression) Div(val interface{}) ArithmeticExpression {
	return arithmeticDiv(sfe, val)
}
