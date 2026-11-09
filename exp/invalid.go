package exp

type invalidExpression struct{ err error }

func newInvalidExpression(err error) InvalidExpression { return invalidExpression{err: err} }

func (ie invalidExpression) Expression() Expression { return ie }

func (ie invalidExpression) Clone() Expression { return ie }

func (ie invalidExpression) Err() error { return ie.err }
