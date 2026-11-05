package exp

type excluded struct {
	col string
}

func NewExcludedExpression(col interface{}) ExcludedExpression {
	var ident IdentifierExpression
	switch c := col.(type) {
	case string:
		ident = ParseIdentifier(c)
	case IdentifierExpression:
		ident = c
	default:
		return excluded{}
	}
	name, _ := ident.GetCol().(string)
	return excluded{col: name}
}

func (e excluded) Expression() Expression { return e }

func (e excluded) Clone() Expression { return e }

func (e excluded) Col() string { return e.col }
