package exp

import "slices"

type IntervalUnit string

const (
	MicrosecondInterval IntervalUnit = "MICROSECOND"
	SecondInterval      IntervalUnit = "SECOND"
	MinuteInterval      IntervalUnit = "MINUTE"
	HourInterval        IntervalUnit = "HOUR"
	DayInterval         IntervalUnit = "DAY"
	WeekInterval        IntervalUnit = "WEEK"
	MonthInterval       IntervalUnit = "MONTH"
	QuarterInterval     IntervalUnit = "QUARTER"
	YearInterval        IntervalUnit = "YEAR"
)

type (
	timeFunctionExpression struct {
		function TimeFunction
		fsp      []int
	}
	intervalExpression struct {
		count any
		unit  IntervalUnit
	}
)

func NewNowExpression(fsp ...int) TimeFunctionExpression {
	return timeFunctionExpression{function: NowFunction, fsp: slices.Clone(fsp)}
}

func NewUTCTimestampExpression(fsp ...int) TimeFunctionExpression {
	return timeFunctionExpression{function: UTCTimestampFunction, fsp: slices.Clone(fsp)}
}

func NewIntervalExpression(count any, unit IntervalUnit) IntervalExpression {
	return intervalExpression{count: count, unit: unit}
}

func (te timeFunctionExpression) Clone() Expression                    { return te }
func (te timeFunctionExpression) Expression() Expression               { return te }
func (te timeFunctionExpression) Function() TimeFunction               { return te.function }
func (te timeFunctionExpression) FSP() []int                           { return slices.Clone(te.fsp) }
func (te timeFunctionExpression) As(val any) AliasedExpression         { return NewAliasExpression(te, val) }
func (te timeFunctionExpression) Eq(val any) BooleanExpression         { return eq(te, val) }
func (te timeFunctionExpression) Neq(val any) BooleanExpression        { return neq(te, val) }
func (te timeFunctionExpression) Gt(val any) BooleanExpression         { return gt(te, val) }
func (te timeFunctionExpression) Gte(val any) BooleanExpression        { return gte(te, val) }
func (te timeFunctionExpression) Lt(val any) BooleanExpression         { return lt(te, val) }
func (te timeFunctionExpression) Lte(val any) BooleanExpression        { return lte(te, val) }
func (te timeFunctionExpression) Add(val any) ArithmeticExpression     { return arithmeticAdd(te, val) }
func (te timeFunctionExpression) Sub(val any) ArithmeticExpression     { return arithmeticSub(te, val) }
func (te timeFunctionExpression) Mul(val any) ArithmeticExpression     { return arithmeticMul(te, val) }
func (te timeFunctionExpression) Div(val any) ArithmeticExpression     { return arithmeticDiv(te, val) }
func (te timeFunctionExpression) Between(val RangeVal) RangeExpression { return between(te, val) }
func (te timeFunctionExpression) NotBetween(val RangeVal) RangeExpression {
	return notBetween(te, val)
}
func (te timeFunctionExpression) In(vals ...any) BooleanExpression    { return in(te, vals...) }
func (te timeFunctionExpression) NotIn(vals ...any) BooleanExpression { return notIn(te, vals...) }
func (te timeFunctionExpression) Is(val any) BooleanExpression        { return is(te, val) }
func (te timeFunctionExpression) IsNot(val any) BooleanExpression     { return isNot(te, val) }
func (te timeFunctionExpression) IsNull() BooleanExpression           { return is(te, nil) }
func (te timeFunctionExpression) IsNotNull() BooleanExpression        { return isNot(te, nil) }
func (te timeFunctionExpression) IsTrue() BooleanExpression           { return is(te, true) }
func (te timeFunctionExpression) IsNotTrue() BooleanExpression        { return isNot(te, true) }
func (te timeFunctionExpression) IsFalse() BooleanExpression          { return is(te, false) }
func (te timeFunctionExpression) IsNotFalse() BooleanExpression       { return isNot(te, false) }
func (te timeFunctionExpression) Asc() OrderedExpression              { return asc(te) }
func (te timeFunctionExpression) Desc() OrderedExpression             { return desc(te) }

func (ie intervalExpression) Clone() Expression      { return ie }
func (ie intervalExpression) Expression() Expression { return ie }
func (ie intervalExpression) Count() any             { return ie.count }
func (ie intervalExpression) Unit() IntervalUnit     { return ie.unit }
