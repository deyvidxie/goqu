package exp

type (
	expressionList struct {
		operator    ExpressionListType
		expressions []Expression
	}
)

func NewExpressionList(operator ExpressionListType, expressions ...Expression) ExpressionList {
	exps := make([]Expression, 0, len(expressions))
	for _, e := range expressions {
		if listType, empty := emptyCondition(e); empty {
			if listType == operator {
				continue
			}
			e = expressionList{operator: listType}
		}
		exps = append(exps, e)
	}
	return expressionList{operator: operator, expressions: exps}
}

func emptyCondition(e Expression) (ExpressionListType, bool) {
	switch t := e.(type) {
	case ExpressionList:
		return t.Type(), t.IsEmpty()
	case Ex:
		return AndType, len(t) == 0
	case ExOr:
		return OrType, len(t) == 0
	case LiteralExpression:
		return AndType, t.Literal() == "" && len(t.Args()) == 0
	default:
		return AndType, false
	}
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
