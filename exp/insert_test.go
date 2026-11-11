package exp_test

import (
	"database/sql/driver"
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/suite"
)

type testAppendableExpression struct {
	exp.AppendableExpression
	sql     string
	args    []interface{}
	clauses exp.SelectClauses
}

func newTestAppendableExpression(sql string, args []interface{}) exp.AppendableExpression {
	return &testAppendableExpression{sql: sql, args: args}
}

func (tae *testAppendableExpression) Expression() exp.Expression {
	return tae
}

func (tae *testAppendableExpression) GetClauses() exp.SelectClauses {
	return tae.clauses
}

func (tae *testAppendableExpression) Clone() exp.Expression {
	return tae
}

type insertExpressionTestSuite struct {
	suite.Suite
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withDifferentRecordTypes() {
	type testRecord struct {
		C string `db:"c"`
	}
	type testRecord2 struct {
		C string `db:"c"`
	}
	_, err := exp.NewInsertExpression(
		"db",
		testRecord{C: "v1"},
		exp.Record{"c": "v2"},
	)
	iets.EqualError(err, "goqu: rows must all be the same type, expected exp_test.testRecord got exp.Record")
	_, err = exp.NewInsertExpression(
		"",
		testRecord{C: "v1"},
		testRecord2{C: "v2"},
	)
	iets.EqualError(err, "goqu: rows must all be the same type, expected exp_test.testRecord got exp_test.testRecord2")
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withRowLists() {
	type testRecord struct {
		C string `db:"c"`
	}
	for name, rows := range map[string]interface{}{
		"array":            [2]testRecord{{C: "a"}, {C: "b"}},
		"pointer to array": &[2]testRecord{{C: "a"}, {C: "b"}},
		"pointer to slice": &[]testRecord{{C: "a"}, {C: "b"}},
		"array of records": [2]exp.Record{{"c": "a"}, {"c": "b"}},
	} {
		ie, err := exp.NewInsertExpression("db", rows)
		iets.Require().NoError(err, name)
		iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols(), name)
		iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals(), name)
	}

	ie, err := exp.NewInsertExpression("db", [0]testRecord{})
	iets.NoError(err)
	iets.True(ie.IsEmpty())

	for _, rows := range []interface{}{(*[2]testRecord)(nil), (*[]testRecord)(nil)} {
		_, err = exp.NewInsertExpression("db", rows)
		iets.EqualError(err, fmt.Sprintf("goqu: insert rows must not be nil, got %T", rows))
		iets.True(errors.IsInvalidExpression(err))
	}

	_, err = exp.NewInsertExpression("db", []byte{1})
	iets.EqualError(err, "goqu: unsupported insert row type []uint8, a map, goqu.Record or struct is required")
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withRowsWithoutColumns() {
	type skipped struct {
		ID int `db:"id" goqu:"skipinsert"`
	}
	var err error
	for name, c := range map[string]struct {
		rows []interface{}
		want string
	}{
		"record": {
			rows: []interface{}{exp.Record{}},
			want: "goqu: insert rows have no columns",
		},
		"records": {
			rows: []interface{}{exp.Record{}, exp.Record{}},
			want: "goqu: insert rows have no columns",
		},
		"structs": {
			rows: []interface{}{[]skipped{{ID: 1}, {ID: 2}, {ID: 3}}},
			want: "goqu: insert rows have no columns",
		},
	} {
		_, err = exp.NewInsertExpression("db", c.rows...)
		iets.EqualError(err, c.want, name)
		iets.True(errors.IsInvalidExpression(err), name)
	}
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withInvalidValue() {
	_, err := exp.NewInsertExpression("db", true)
	iets.EqualError(err, "goqu: unsupported insert row type bool, a map, goqu.Record or struct is required")
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withDifferentTypes() {
	_, err := exp.NewInsertExpression("db", exp.Record{"a": "a1"}, true)
	iets.EqualError(err, "goqu: rows must all be the same type, expected exp.Record got bool")
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withNoValues() {
	ie, err := exp.NewInsertExpression("db")
	iets.NoError(err)
	iets.Nil(ie.Cols())
	iets.Nil(ie.Vals())
	iets.True(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

type insertValuerRow struct {
	A int `db:"a"`
}

type insertValuerRows []insertValuerRow

func (insertValuerRows) Value() (driver.Value, error) { return "[]", nil }

func (iets *insertExpressionTestSuite) TestNewInsertExpression_expandsASliceValuerOfRows() {
	rows := insertValuerRows{{A: 1}, {A: 2}}
	for _, source := range []interface{}{rows, &rows} {
		ie, err := exp.NewInsertExpression("db", source)
		iets.Require().NoError(err)
		iets.Equal([][]interface{}{{1}, {2}}, ie.Vals())
	}
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_Vals() {
	ie, err := exp.NewInsertExpression("db")
	iets.NoError(err)
	vals := [][]interface{}{
		{"a", "b"},
	}
	ie = ie.SetCols(exp.NewColumnListExpression(nil, "db", "a", "b")).SetVals(vals)
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
	iets.Equal(vals, ie.Vals())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_Cols() {
	ie, err := exp.NewInsertExpression("db")
	iets.NoError(err)
	vals := [][]interface{}{
		{"a", "b"},
	}
	ce := exp.NewColumnListExpression(nil, "db", "a", "b")
	ie = ie.SetCols(ce).SetVals(vals)
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
	iets.Equal(vals, ie.Vals())
	iets.Equal(ce, ie.Cols())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_From() {
	ae := newTestAppendableExpression("select * from test", []interface{}{})
	ie, err := exp.NewInsertExpression("db", ae)
	iets.NoError(err)
	iets.False(ie.IsEmpty())
	iets.True(ie.IsInsertFrom())
	iets.Equal(ae, ie.From())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_appendableExpression() {
	ae := newTestAppendableExpression("test ae", nil)

	ie, err := exp.NewInsertExpression("db", ae)
	iets.NoError(err)
	iets.False(ie.IsEmpty())
	iets.True(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withRecords() {
	ie, err := exp.NewInsertExpression("db", exp.Record{"c": "a"}, exp.Record{"c": "b"})
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols())
	iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withRecordsSlice() {
	ie, err := exp.NewInsertExpression("db", []exp.Record{{"c": "a"}, {"c": "b"}})
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols())
	iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withRecordOfDifferentLength() {
	_, err := exp.NewInsertExpression("db", exp.Record{"c": "a"}, exp.Record{"c": "b", "c2": "d"})
	iets.EqualError(err, "goqu: rows with different value length, expected 1 got 2")
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withRecordWithDifferentkeys() {
	_, err := exp.NewInsertExpression("db", exp.Record{"c1": "a"}, exp.Record{"c2": "b"})
	iets.EqualError(err, `goqu: rows with different keys, expected ["c1"] got ["c2"]`)
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withMap() {
	ie, err := exp.NewInsertExpression(
		"db",
		map[string]interface{}{"c": "a"},
		map[string]interface{}{"c": "b"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols())
	iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructs() {
	type testRecord struct {
		C string `db:"c"`
	}
	ie, err := exp.NewInsertExpression(
		"db",
		testRecord{C: "a"},
		testRecord{C: "b"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols())
	iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructSlice() {
	type testRecord struct {
		C string `db:"c"`
	}
	ie, err := exp.NewInsertExpression("db", []testRecord{
		{C: "a"},
		{C: "b"},
	})
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols())
	iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructsWithoutTags() {
	type testRecord struct {
		FieldA int64
		FieldB bool
		FieldC string
	}
	ie, err := exp.NewInsertExpression(
		"db",
		testRecord{FieldA: 1, FieldB: true, FieldC: "a"},
		testRecord{FieldA: 2, FieldB: false, FieldC: "b"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "fielda", "fieldb", "fieldc"), ie.Cols())
	iets.Equal([][]interface{}{{int64(1), true, "a"}, {int64(2), false, "b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructsIgnoredDbTag() {
	type testRecord struct {
		FieldA int64 `db:"-"`
		FieldB bool
		FieldC string
	}
	ie, err := exp.NewInsertExpression(
		"db",
		testRecord{FieldA: 1, FieldB: true, FieldC: "a"},
		testRecord{FieldA: 2, FieldB: false, FieldC: "b"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "fieldb", "fieldc"), ie.Cols())
	iets.Equal([][]interface{}{{true, "a"}, {false, "b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructsWithGoquSkipInsert() {
	type testRecord struct {
		FieldA int64
		FieldB bool   `goqu:"skipupdate"`
		FieldC string `goqu:"skipinsert"`
	}
	ie, err := exp.NewInsertExpression(
		"db",
		testRecord{FieldA: 1, FieldB: true, FieldC: "a"},
		testRecord{FieldA: 2, FieldB: false, FieldC: "b"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "fielda", "fieldb"), ie.Cols())
	iets.Equal([][]interface{}{{int64(1), true}, {int64(2), false}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructPointers() {
	type testRecord struct {
		C string `db:"c"`
	}
	ie, err := exp.NewInsertExpression(
		"db",
		&testRecord{C: "a"},
		&testRecord{C: "b"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "c"), ie.Cols())
	iets.Equal([][]interface{}{{"a"}, {"b"}}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructsWithEmbeddedStructs() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	ie, err := exp.NewInsertExpression(
		"db",
		item{Address: "111 Test Addr", Name: "Test1", Phone: Phone{Home: "123123", Primary: "456456"}},
		item{Address: "211 Test Addr", Name: "Test2", Phone: Phone{Home: "123123", Primary: "456456"}},
		item{Address: "311 Test Addr", Name: "Test3", Phone: Phone{Home: "123123", Primary: "456456"}},
		item{Address: "411 Test Addr", Name: "Test4", Phone: Phone{Home: "123123", Primary: "456456"}},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "address", "home_phone", "name", "primary_phone"), ie.Cols())
	iets.Equal([][]interface{}{
		{"111 Test Addr", "123123", "Test1", "456456"},
		{"211 Test Addr", "123123", "Test2", "456456"},
		{"311 Test Addr", "123123", "Test3", "456456"},
		{"411 Test Addr", "123123", "Test4", "456456"},
	}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withStructsWithEmbeddedStructPointers() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		*Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	ie, err := exp.NewInsertExpression(
		"db",
		item{Address: "111 Test Addr", Name: "Test1", Phone: &Phone{Home: "123123", Primary: "456456"}},
		item{Address: "211 Test Addr", Name: "Test2", Phone: &Phone{Home: "123123", Primary: "456456"}},
		item{Address: "311 Test Addr", Name: "Test3", Phone: &Phone{Home: "123123", Primary: "456456"}},
		item{Address: "411 Test Addr", Name: "Test4", Phone: &Phone{Home: "123123", Primary: "456456"}},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "address", "home_phone", "name", "primary_phone"), ie.Cols())
	iets.Equal([][]interface{}{
		{"111 Test Addr", "123123", "Test1", "456456"},
		{"211 Test Addr", "123123", "Test2", "456456"},
		{"311 Test Addr", "123123", "Test3", "456456"},
		{"411 Test Addr", "123123", "Test4", "456456"},
	}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withNilEmbeddedStructPointers() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		*Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	ie, err := exp.NewInsertExpression(
		"db",
		item{Address: "111 Test Addr", Name: "Test1"},
		item{Address: "211 Test Addr", Name: "Test2"},
		item{Address: "311 Test Addr", Name: "Test3"},
		item{Address: "411 Test Addr", Name: "Test4"},
	)
	iets.NoError(err)
	iets.Equal(exp.NewColumnListExpression(nil, "db", "address", "name"), ie.Cols())
	iets.Equal([][]interface{}{
		{"111 Test Addr", "Test1"},
		{"211 Test Addr", "Test2"},
		{"311 Test Addr", "Test3"},
		{"411 Test Addr", "Test4"},
	}, ie.Vals())
	iets.False(ie.IsEmpty())
	iets.False(ie.IsInsertFrom())
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withDifferentStructTypes() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type item struct {
		*Phone
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	_, err := exp.NewInsertExpression(
		"db",
		item{Address: "111 Test Addr", Name: "Test1"},
		Phone{Home: "123123", Primary: "456456"},
		item{Address: "311 Test Addr", Name: "Test3"},
		Phone{Home: "123123", Primary: "456456"},
	)
	iets.EqualError(err, "goqu: rows must all be the same type, expected exp_test.item got exp_test.Phone")
}

func (iets *insertExpressionTestSuite) TestNewInsertExpression_withDifferentColumnLengths() {
	type Phone struct {
		Primary string `db:"primary_phone"`
		Home    string `db:"home_phone"`
	}
	type Phone2 struct {
		Primary string `db:"primary_phone2"`
		Home    string `db:"home_phone2"`
	}
	type item struct {
		*Phone
		*Phone2
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	_, err := exp.NewInsertExpression(
		"db",
		item{Address: "111 Test Addr", Name: "Test1", Phone2: &Phone2{Home: "123123", Primary: "456456"}},
		item{Address: "311 Test Addr", Name: "Test3", Phone: &Phone{Home: "123123", Primary: "456456"}},
	)
	iets.EqualError(err, `goqu: rows with different keys, expected `+
		`["address","home_phone2","name","primary_phone2"] got ["address","home_phone","name","primary_phone"]`)
}

func TestInsertExpressionSuite(t *testing.T) {
	suite.Run(t, new(insertExpressionTestSuite))
}
