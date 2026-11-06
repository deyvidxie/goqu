package exp

type (
	JSONFunction int

	jsonExpression struct {
		function  JSONFunction
		col       interface{}
		candidate interface{}
		paths     []string
	}
)

const (
	JSONExtractFunction JSONFunction = iota
	JSONValueFunction
	JSONContainsFunction
)

func NewJSONExtractExpression(col interface{}, path string) JSONExpression {
	return jsonExpression{function: JSONExtractFunction, col: jsonCol(col), paths: []string{path}}
}

func NewJSONValueExpression(col interface{}, path string) JSONExpression {
	return jsonExpression{function: JSONValueFunction, col: jsonCol(col), paths: []string{path}}
}

func NewJSONContainsExpression(col, candidate interface{}, paths ...string) JSONExpression {
	return jsonExpression{function: JSONContainsFunction, col: jsonCol(col), candidate: candidate, paths: paths}
}

func jsonCol(col interface{}) interface{} {
	if s, ok := col.(string); ok {
		return ParseIdentifier(s)
	}
	return col
}

func (je jsonExpression) Clone() Expression                    { return je }
func (je jsonExpression) Expression() Expression               { return je }
func (je jsonExpression) Function() JSONFunction               { return je.function }
func (je jsonExpression) Col() interface{}                     { return je.col }
func (je jsonExpression) Candidate() interface{}               { return je.candidate }
func (je jsonExpression) Paths() []string                      { return je.paths }
func (je jsonExpression) As(val interface{}) AliasedExpression { return NewAliasExpression(je, val) }

func (je jsonExpression) Eq(val interface{}) BooleanExpression  { return eq(je, val) }
func (je jsonExpression) Neq(val interface{}) BooleanExpression { return neq(je, val) }
func (je jsonExpression) Gt(val interface{}) BooleanExpression  { return gt(je, val) }
func (je jsonExpression) Gte(val interface{}) BooleanExpression { return gte(je, val) }
func (je jsonExpression) Lt(val interface{}) BooleanExpression  { return lt(je, val) }
func (je jsonExpression) Lte(val interface{}) BooleanExpression { return lte(je, val) }

func (je jsonExpression) Between(val RangeVal) RangeExpression    { return between(je, val) }
func (je jsonExpression) NotBetween(val RangeVal) RangeExpression { return notBetween(je, val) }

func (je jsonExpression) In(vals ...interface{}) BooleanExpression    { return in(je, vals...) }
func (je jsonExpression) NotIn(vals ...interface{}) BooleanExpression { return notIn(je, vals...) }

func (je jsonExpression) Is(val interface{}) BooleanExpression    { return is(je, val) }
func (je jsonExpression) IsNot(val interface{}) BooleanExpression { return isNot(je, val) }
func (je jsonExpression) IsNull() BooleanExpression               { return is(je, nil) }
func (je jsonExpression) IsNotNull() BooleanExpression            { return isNot(je, nil) }
func (je jsonExpression) IsTrue() BooleanExpression               { return is(je, true) }
func (je jsonExpression) IsNotTrue() BooleanExpression            { return isNot(je, true) }
func (je jsonExpression) IsFalse() BooleanExpression              { return is(je, false) }
func (je jsonExpression) IsNotFalse() BooleanExpression           { return isNot(je, false) }

func (je jsonExpression) Like(val interface{}) BooleanExpression       { return like(je, val) }
func (je jsonExpression) NotLike(val interface{}) BooleanExpression    { return notLike(je, val) }
func (je jsonExpression) ILike(val interface{}) BooleanExpression      { return iLike(je, val) }
func (je jsonExpression) NotILike(val interface{}) BooleanExpression   { return notILike(je, val) }
func (je jsonExpression) RegexpLike(val interface{}) BooleanExpression { return regexpLike(je, val) }
func (je jsonExpression) RegexpNotLike(val interface{}) BooleanExpression {
	return regexpNotLike(je, val)
}
func (je jsonExpression) RegexpILike(val interface{}) BooleanExpression { return regexpILike(je, val) }
func (je jsonExpression) RegexpNotILike(val interface{}) BooleanExpression {
	return regexpNotILike(je, val)
}

func (je jsonExpression) Asc() OrderedExpression  { return asc(je) }
func (je jsonExpression) Desc() OrderedExpression { return desc(je) }
