package exp

type defaultValue struct{}

func Default() DefaultExpression { return defaultValue{} }

func (defaultValue) Expression() Expression { return defaultValue{} }

func (defaultValue) Clone() Expression { return defaultValue{} }

func (defaultValue) isDefault() {}
