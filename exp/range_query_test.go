package exp_test

import (
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/suite"
)

type rangeQueryTest struct {
	suite.Suite
}

func TestRangeQuerySuite(t *testing.T) {
	suite.Run(t, new(rangeQueryTest))
}

func (rqt *rangeQueryTest) TestRangeFill() {
	rqt.Equal(exp.RangeFillLinear, exp.NewRangeFill(exp.RangeFillLinear).Kind())
	fill := exp.NewRangeFillValue(1.5)
	rqt.Equal(exp.RangeFillValue, fill.Kind())
	rqt.Equal(1.5, fill.Value())
}

func (rqt *rangeQueryTest) TestRangeAggregate() {
	agg := exp.NewSQLFunctionExpression("avg", exp.NewIdentifierExpression("", "", "cpu"))
	ra := exp.NewRangeAggregateExpression(agg, time.Minute)
	withFill := ra.Fill(exp.NewRangeFill(exp.RangeFillPrev))

	rqt.Equal(agg, ra.Aggregate())
	rqt.Equal(time.Minute, ra.Duration())
	_, ok := ra.GetFill()
	rqt.False(ok)
	fill, ok := withFill.GetFill()
	rqt.True(ok)
	rqt.Equal(exp.RangeFillPrev, fill.Kind())

	rqt.Equal(ra, ra.Expression())
	rqt.Equal(ra, ra.Clone())
	rqt.Equal(exp.NewAliasExpression(ra, "a"), ra.As("a"))
}

func (rqt *rangeQueryTest) TestRangeAggregateWithoutAggregate() {
	invalid, ok := exp.NewRangeAggregateExpression(nil, time.Minute).Aggregate().(exp.InvalidExpression)
	rqt.Require().True(ok)
	rqt.EqualError(invalid.Err(), "goqu: RangeAggregate requires an aggregate expression")
}

func (rqt *rangeQueryTest) TestAlign() {
	a := exp.NewAlignExpression(time.Second)
	origin := time.Unix(10, 0)

	rqt.Equal(time.Second, a.Duration())
	rqt.Equal(a, a.Expression())
	_, ok := a.ToTime()
	rqt.False(ok)
	rqt.False(a.IsToNow())
	rqt.Nil(a.ByCols())
	_, ok = a.GetFill()
	rqt.False(ok)

	to, ok := a.To(origin).ToTime()
	rqt.True(ok)
	rqt.Equal(origin, to)
	rqt.True(a.To(origin).ToNow().IsToNow())
	_, ok = a.ToNow().ToTime()
	rqt.False(ok)
	rqt.False(a.ToNow().To(origin).IsToNow())

	rqt.Equal(exp.NewColumnListExpression(nil, "", "host", "dc"), a.By("host", "dc").ByCols())
	rqt.True(a.ByNone().ByCols().IsEmpty())
	fill, ok := a.Fill(exp.NewRangeFillValue(2)).GetFill()
	rqt.True(ok)
	rqt.Equal(2.0, fill.Value())
}

func (rqt *rangeQueryTest) TestAlignByWithoutColumns() {
	cols := exp.NewAlignExpression(time.Second).By().ByCols().Columns()
	rqt.Require().Len(cols, 1)
	invalid, ok := cols[0].(exp.InvalidExpression)
	rqt.Require().True(ok)
	rqt.EqualError(invalid.Err(), "goqu: Align By requires at least one column; use ByNone for BY ()")
}

func (rqt *rangeQueryTest) TestAlignClone() {
	a := exp.NewAlignExpression(time.Second).By("host")
	rqt.Equal(a, a.Clone())
	rqt.Equal(exp.NewAlignExpression(time.Second), exp.NewAlignExpression(time.Second).Clone())
}
