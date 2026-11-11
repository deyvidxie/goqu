package exp

type invalidExpression struct{ err error }

func newInvalidExpression(err error) InvalidExpression { return invalidExpression{err: err} }

func (ie invalidExpression) Expression() Expression { return ie }

func (ie invalidExpression) Clone() Expression { return ie }

func (ie invalidExpression) Err() error { return ie.err }

func (invalidExpression) isInvalid() {}

func emptyAppendList(cl ColumnListExpression, method, noun string) ColumnListExpression {
	invalid := newInvalidExpression(errEmptyAppend(method, noun))
	if cl == nil {
		return NewColumnListExpression(nil, "", invalid)
	}
	return cl.Append(invalid)
}

func emptyAppendExpressions(method string) []Expression {
	return []Expression{newInvalidExpression(errEmptyAppend(method, "expression"))}
}

func isSetButEmpty(cl ColumnListExpression) bool {
	return cl != nil && cl.IsEmpty()
}

func emptyAppendRow(method string) []interface{} {
	return []interface{}{newInvalidExpression(errEmptyAppend(method, "row"))}
}
