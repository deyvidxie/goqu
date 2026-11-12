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
	rqt.False(ra.HasFill())
	rqt.Equal(exp.RangeFill{}, ra.GetFill())
	rqt.True(withFill.HasFill())
	rqt.Equal(exp.RangeFillPrev, withFill.GetFill().Kind())

	rqt.Equal(ra, ra.Expression())
	rqt.Equal(ra, ra.Clone())
	rqt.Equal(exp.NewAliasExpression(ra, "a"), ra.As("a"))
	rqt.Equal(exp.NewOrderedExpression(ra, exp.AscDir, exp.NoNullsSortType), ra.Asc())
	rqt.Equal(exp.NewOrderedExpression(ra, exp.DescSortDir, exp.NoNullsSortType), ra.Desc())
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
	rqt.False(a.HasTo())
	rqt.False(a.IsToNow())
	rqt.True(a.ToTime().IsZero())
	rqt.False(a.HasBy())
	rqt.Nil(a.ByCols())
	rqt.False(a.HasFill())
	rqt.Equal(exp.RangeFill{}, a.GetFill())

	rqt.True(a.To(origin).HasTo())
	rqt.False(a.To(origin).IsToNow())
	rqt.Equal(origin, a.To(origin).ToTime())
	rqt.True(a.ToNow().HasTo())
	rqt.True(a.To(origin).ToNow().IsToNow())
	rqt.True(a.ToNow().ToTime().IsZero())
	rqt.False(a.ToNow().To(origin).IsToNow())

	rqt.True(a.By("host").HasBy())
	rqt.Equal(exp.NewColumnListExpression(nil, "", "host", "dc"), a.By("host", "dc").ByCols())
	rqt.True(a.ByNone().HasBy())
	rqt.True(a.ByNone().ByCols().IsEmpty())
	rqt.True(a.Fill(exp.NewRangeFillValue(2)).HasFill())
	rqt.Equal(2.0, a.Fill(exp.NewRangeFillValue(2)).GetFill().Value())
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
