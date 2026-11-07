package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/suite"
)

type updateExpressionTestSuite struct {
	suite.Suite
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withNil() {
	_, err := exp.NewUpdateExpressions("db", nil)
	uets.EqualError(err, "goqu: update values must not be nil, got <nil>")
	uets.True(errors.IsInvalidExpression(err))
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withInvalidValue() {
	_, err := exp.NewUpdateExpressions("db", true)
	uets.EqualError(err, "goqu: unsupported update interface type bool")
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withRecords() {
	ie, err := exp.NewUpdateExpressions("db", exp.Record{"c": "a", "b": "d"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "b").Set("d"),
		exp.NewIdentifierExpression("", "", "c").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withMap() {
	ie, err := exp.NewUpdateExpressions("db", map[string]interface{}{"c": "a", "b": "d"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "b").Set("d"),
		exp.NewIdentifierExpression("", "", "c").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructs() {
	type testRecord struct {
		C string `db:"c"`
		B string `db:"b"`
	}
	ie, err := exp.NewUpdateExpressions("db", testRecord{C: "a", B: "d"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "b").Set("d"),
		exp.NewIdentifierExpression("", "", "c").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructsWithoutTags() {
	type testRecord struct {
		FieldA int64
		FieldB bool
		FieldC string
	}
	ie, err := exp.NewUpdateExpressions("db", testRecord{FieldA: 1, FieldB: true, FieldC: "a"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "fielda").Set(int64(1)),
		exp.NewIdentifierExpression("", "", "fieldb").Set(true),
		exp.NewIdentifierExpression("", "", "fieldc").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructsIgnoredDbTag() {
	type testRecord struct {
		FieldA int64 `db:"-"`
		FieldB bool
		FieldC string
	}
	ie, err := exp.NewUpdateExpressions("db", testRecord{FieldA: 1, FieldB: true, FieldC: "a"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "fieldb").Set(true),
		exp.NewIdentifierExpression("", "", "fieldc").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructsWithGoquSkipUpdate() {
	type testRecord struct {
		FieldA int64
		FieldB bool   `goqu:"skipupdate"`
		FieldC string `goqu:"skipinsert"`
	}
	ie, err := exp.NewUpdateExpressions("db", testRecord{FieldA: 1, FieldB: true, FieldC: "a"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "fielda").Set(int64(1)),
		exp.NewIdentifierExpression("", "", "fieldc").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructPointers() {
	type testRecord struct {
		C string `db:"c"`
		B string `db:"b"`
	}
	ie, err := exp.NewUpdateExpressions("db", &testRecord{C: "a", B: "d"})
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "b").Set("d"),
		exp.NewIdentifierExpression("", "", "c").Set("a"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructsWithEmbeddedStructs() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	ie, err := exp.NewUpdateExpressions(
		"db",
		item{Address: "111 Test Addr", Name: "Test1", Phone: Phone{Home: "123123", Primary: "456456"}},
	)
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "address").Set("111 Test Addr"),
		exp.NewIdentifierExpression("", "", "home_phone").Set("123123"),
		exp.NewIdentifierExpression("", "", "name").Set("Test1"),
		exp.NewIdentifierExpression("", "", "primary_phone").Set("456456"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withStructsWithEmbeddedStructPointers() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		*Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	ie, err := exp.NewUpdateExpressions(
		"db",
		item{Address: "111 Test Addr", Name: "Test1", Phone: &Phone{Home: "123123", Primary: "456456"}},
	)
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "address").Set("111 Test Addr"),
		exp.NewIdentifierExpression("", "", "home_phone").Set("123123"),
		exp.NewIdentifierExpression("", "", "name").Set("Test1"),
		exp.NewIdentifierExpression("", "", "primary_phone").Set("456456"),
	}
	uets.Equal(eie, ie)
}

func (uets *updateExpressionTestSuite) TestNewUpdateExpressions_withNilEmbeddedStructPointers() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		*Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	ie, err := exp.NewUpdateExpressions(
		"db",
		item{Address: "111 Test Addr", Name: "Test1"},
	)
	uets.NoError(err)
	eie := []exp.UpdateExpression{
		exp.NewIdentifierExpression("", "", "address").Set("111 Test Addr"),
		exp.NewIdentifierExpression("", "", "name").Set("Test1"),
	}
	uets.Equal(eie, ie)
}

func TestUpdateExpressionSuite(t *testing.T) {
	suite.Run(t, new(updateExpressionTestSuite))
}
