package exp

type sqlWindowExpression struct {
	name          IdentifierExpression
	parent        IdentifierExpression
	partitionCols ColumnListExpression
	orderCols     ColumnListExpression
}

func NewWindowExpression(window, parent IdentifierExpression, partitionCols, orderCols ColumnListExpression) WindowExpression {
	return sqlWindowExpression{
		name:          window,
		parent:        parent,
		partitionCols: partitionCols,
		orderCols:     orderCols,
	}
}

func (we sqlWindowExpression) clone() sqlWindowExpression {
	return sqlWindowExpression{
		name:          we.name,
		parent:        we.parent,
		partitionCols: cloneColumnList(we.partitionCols),
		orderCols:     cloneColumnList(we.orderCols),
	}
}

func cloneColumnList(cl ColumnListExpression) ColumnListExpression {
	if cl == nil {
		return nil
	}
	return cl.Clone().(ColumnListExpression)
}

func (we sqlWindowExpression) Clone() Expression {
	return we.clone()
}

func (we sqlWindowExpression) Expression() Expression {
	return we
}

func (we sqlWindowExpression) Name() IdentifierExpression {
	return we.name
}

func (we sqlWindowExpression) HasName() bool {
	return we.name != nil
}

func (we sqlWindowExpression) Parent() IdentifierExpression {
	return we.parent
}

func (we sqlWindowExpression) HasParent() bool {
	return we.parent != nil
}

func (we sqlWindowExpression) PartitionCols() ColumnListExpression {
	return we.partitionCols
}

func (we sqlWindowExpression) HasPartitionBy() bool {
	return we.partitionCols != nil
}

func (we sqlWindowExpression) OrderCols() ColumnListExpression {
	return we.orderCols
}

func (we sqlWindowExpression) HasOrder() bool {
	return we.orderCols != nil
}

func (we sqlWindowExpression) PartitionBy(cols ...interface{}) WindowExpression {
	ret := we.clone()
	ret.partitionCols = windowColumns("PartitionBy", cols)
	return ret
}

func (we sqlWindowExpression) OrderBy(cols ...interface{}) WindowExpression {
	ret := we.clone()
	ret.orderCols = windowColumns("OrderBy", cols)
	return ret
}

func (we sqlWindowExpression) Inherit(parent string) WindowExpression {
	ret := we.clone()
	ret.parent = ParseIdentifier(parent)
	return ret
}

func windowColumns(method string, cols []interface{}) ColumnListExpression {
	cl := newRenderTaggedColumnList(cols...)
	if cl.IsEmpty() {
		return emptyAppendList(nil, method, "column")
	}
	return cl
}
