package exp

import "time"

type (
	RangeFillKind int

	RangeFill struct {
		kind  RangeFillKind
		value float64
	}

	rangeAggregate struct {
		aggregate Expression
		duration  time.Duration
		fill      *RangeFill
	}

	alignOrigin struct {
		now  bool
		time time.Time
	}

	align struct {
		duration time.Duration
		to       *alignOrigin
		by       ColumnListExpression
		fill     *RangeFill
	}
)

const (
	RangeFillNull RangeFillKind = iota + 1
	RangeFillPrev
	RangeFillLinear
	RangeFillValue
)

func NewRangeFill(kind RangeFillKind) RangeFill { return RangeFill{kind: kind} }

func NewRangeFillValue(value float64) RangeFill { return RangeFill{kind: RangeFillValue, value: value} }

func (f RangeFill) Kind() RangeFillKind { return f.kind }

func (f RangeFill) Value() float64 { return f.value }

func NewRangeAggregateExpression(aggregate Expression, duration time.Duration) RangeAggregateExpression {
	if aggregate == nil {
		aggregate = newInvalidExpression(errNilRangeAggregate)
	}
	return rangeAggregate{aggregate: aggregate, duration: duration}
}

func (ra rangeAggregate) Clone() Expression {
	ra.aggregate = ra.aggregate.Clone()
	return ra
}

func (ra rangeAggregate) Expression() Expression { return ra }

func (ra rangeAggregate) As(val interface{}) AliasedExpression { return NewAliasExpression(ra, val) }

func (ra rangeAggregate) Asc() OrderedExpression { return asc(ra) }

func (ra rangeAggregate) Desc() OrderedExpression { return desc(ra) }

func (ra rangeAggregate) Aggregate() Expression { return ra.aggregate }

func (ra rangeAggregate) Duration() time.Duration { return ra.duration }

func (ra rangeAggregate) HasFill() bool { return ra.fill != nil }

func (ra rangeAggregate) GetFill() RangeFill { return fillOrZero(ra.fill) }

func (ra rangeAggregate) Fill(fill RangeFill) RangeAggregateExpression {
	ra.fill = &fill
	return ra
}

func NewAlignExpression(duration time.Duration) AlignExpression { return align{duration: duration} }

func (a align) Clone() Expression {
	if a.by != nil {
		a.by = a.by.Clone().(ColumnListExpression)
	}
	return a
}

func (a align) Expression() Expression { return a }

func (a align) Duration() time.Duration { return a.duration }

func (a align) HasTo() bool { return a.to != nil }

func (a align) IsToNow() bool { return a.to != nil && a.to.now }

func (a align) ToTime() time.Time {
	if a.to == nil {
		return time.Time{}
	}
	return a.to.time
}

func (a align) HasBy() bool { return a.by != nil }

func (a align) ByCols() ColumnListExpression { return a.by }

func (a align) HasFill() bool { return a.fill != nil }

func (a align) GetFill() RangeFill { return fillOrZero(a.fill) }

func (a align) To(origin time.Time) AlignExpression {
	a.to = &alignOrigin{time: origin}
	return a
}

func (a align) ToNow() AlignExpression {
	a.to = &alignOrigin{now: true}
	return a
}

func (a align) By(cols ...interface{}) AlignExpression {
	a.by = newRenderTaggedColumnList(cols...)
	if a.by.IsEmpty() {
		a.by = NewColumnListExpression(nil, "", newInvalidExpression(errEmptyAlignBy))
	}
	return a
}

func (a align) ByNone() AlignExpression {
	a.by = newRenderTaggedColumnList()
	return a
}

func (a align) Fill(fill RangeFill) AlignExpression {
	a.fill = &fill
	return a
}

func fillOrZero(fill *RangeFill) RangeFill {
	if fill == nil {
		return RangeFill{}
	}
	return *fill
}
