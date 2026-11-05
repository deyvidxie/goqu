package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestNewExcludedExpression(t *testing.T) {
	cases := []struct {
		name string
		col  interface{}
		want string
	}{
		{name: "column string", col: "a", want: "a"},
		{name: "qualified string", col: "t.a", want: "a"},
		{name: "schema qualified string", col: "s.t.a", want: "a"},
		{name: "column identifier", col: exp.NewIdentifierExpression("", "", "a"), want: "a"},
		{name: "qualified identifier", col: exp.NewIdentifierExpression("s", "t", "a"), want: "a"},
		{name: "identifier without column", col: exp.NewIdentifierExpression("", "t", nil), want: ""},
		{name: "non string column", col: exp.NewIdentifierExpression("", "", exp.Star()), want: ""},
		{name: "unsupported type", col: 1, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := exp.NewExcludedExpression(tc.col)
			assert.Equal(t, tc.want, e.Col())
			assert.Equal(t, e, e.Expression())
			assert.Equal(t, e, e.Clone())
		})
	}
}
