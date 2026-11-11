package exp

import "slices"

type (
	expressionList struct {
		operator    ExpressionListType
		expressions []Expression
	}
)

func NewExpressionList(operator ExpressionListType, expressions ...Expression) ExpressionList {
	exps := slices.Clone(expressions)
	for i, e := range exps {
		if e == nil {
			exps[i] = newInvalidExpression(errNilListEntry)
		}
	}
	return expressionList{operator: operator, expressions: exps}
}

func (el expressionList) Clone() Expression {
	newExps := make([]Expression, 0, len(el.expressions))
	for _, exp := range el.expressions {
		newExps = append(newExps, exp.Clone())
	}
	return expressionList{operator: el.operator, expressions: newExps}
}

func (el expressionList) Expression() Expression {
	return el
}

func (el expressionList) IsEmpty() bool {
	return len(el.expressions) == 0
}

func (el expressionList) Type() ExpressionListType {
	return el.operator
}

func (el expressionList) Expressions() []Expression {
	return el.expressions
}

func (el expressionList) Append(expressions ...Expression) ExpressionList {
	exps := make([]Expression, 0, len(el.expressions)+len(expressions))
	exps = append(exps, el.expressions...)
	exps = append(exps, expressions...)
	return NewExpressionList(el.operator, exps...)
}
