package goqu_test

import (
	"context"
	"database/sql"
	"iter"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
)

func collect[T any](seq iter.Seq2[T, error]) (rows []T, errs []error) {
	for row, err := range seq {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		rows = append(rows, row)
	}
	return rows, errs
}

func (ss *scanSuite) TestIterStructs() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2")).
		RowsWillBeClosed()

	items, errs := collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestIterStructs_pointers() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2"))

	items, errs := collect(ss.db.From("items").IterStructs[*dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Equal([]*dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}, {Address: "211 Test Addr", Name: "Test2"}}, items)
}

func (ss *scanSuite) TestIterStructs_breakClosesRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2")).
		RowsWillBeClosed()

	var items []dsTestActionItem
	for item, err := range ss.db.From("items").IterStructs[dsTestActionItem](context.Background()) {
		ss.Require().NoError(err)
		items = append(items, item)
		break
	}
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestIterStructs_runsTheQueryOnEveryRange() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("111 Test Addr,Test1"))
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("211 Test Addr,Test2"))

	seq := ss.db.From("items").IterStructs[dsTestActionItem](context.Background())
	first, firstErrs := collect(seq)
	second, secondErrs := collect(seq)
	ss.Empty(append(firstErrs, secondErrs...))
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, first)
	ss.Equal([]dsTestActionItem{{Address: "211 Test Addr", Name: "Test2"}}, second)
}

func (ss *scanSuite) TestIterStructs_noRows() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnRows(ss.itemRows("")).RowsWillBeClosed()

	items, errs := collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Empty(items)
}

func (ss *scanSuite) TestIterStructs_queryError() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).WillReturnError(scanTestError{})

	items, errs := collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(items)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterStructs_rowErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1\n211 Test Addr,Test2").RowError(1, scanTestError{})).
		RowsWillBeClosed()

	items, errs := collect(ss.db.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterStructs_scanErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "test" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("a\nb")).
		RowsWillBeClosed()

	items, errs := collect(ss.db.From("items").Select("test").IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(items)
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsColumnMismatch(errs[0]))
}

func (ss *scanSuite) TestIterStructs_requiresStruct() {
	items, errs := collect(ss.db.From("items").IterStructs[sql.NullString](context.Background()))
	ss.Empty(items)
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsUnsupportedScanType(errs[0]))
}

func (ss *scanSuite) TestIterStructs_fromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("address", "name") VALUES ('111 Test Addr', 'Test1') RETURNING "address", "name"`).
		WillReturnRows(ss.itemRows("111 Test Addr,Test1"))

	ds := ss.db.Insert("items").Rows(dsTestActionItem{Address: "111 Test Addr", Name: "Test1"}).Returning("address", "name")
	items, errs := collect(ds.Executor().IterStructs[dsTestActionItem](context.Background()))
	ss.Empty(errs)
	ss.Equal([]dsTestActionItem{{Address: "111 Test Addr", Name: "Test1"}}, items)
}

func (ss *scanSuite) TestIterStructs_withoutDatabase() {
	_, errs := collect(goqu.From("items").IterStructs[dsTestActionItem](context.Background()))
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsNoDatabase(errs[0]))
}

func (ss *scanSuite) TestIterVals() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2")).
		RowsWillBeClosed()

	ids, errs := collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Empty(errs)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestIterVals_nullablePointers() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(nil))

	ids, errs := collect(ss.db.From("items").Select("id").IterVals[*int64](context.Background()))
	ss.Empty(errs)
	ss.Require().Len(ids, 2)
	ss.Equal(int64(1), *ids[0])
	ss.Nil(ids[1])
}

func (ss *scanSuite) TestIterVals_breakClosesRows() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2")).
		RowsWillBeClosed()

	var ids []int64
	for id, err := range ss.db.From("items").Select("id").IterVals[int64](context.Background()) {
		ss.Require().NoError(err)
		ids = append(ids, id)
		break
	}
	ss.Equal([]int64{1}, ids)
}

func (ss *scanSuite) TestIterVals_queryError() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).WillReturnError(scanTestError{})

	ids, errs := collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Empty(ids)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterVals_rowErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2").RowError(1, scanTestError{})).
		RowsWillBeClosed()

	ids, errs := collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Equal([]int64{1}, ids)
	ss.Require().Len(errs, 1)
	ss.ErrorIs(errs[0], scanTestError{})
}

func (ss *scanSuite) TestIterVals_scanErrorEndsTheIteration() {
	ss.mock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("a\nb")).
		RowsWillBeClosed()

	ids, errs := collect(ss.db.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Empty(ids)
	ss.Len(errs, 1)
}

func (ss *scanSuite) iterValsErr[T any]() error {
	vals, errs := collect(ss.db.From("items").Select("id").IterVals[T](context.Background()))
	ss.Empty(vals)
	ss.Require().Len(errs, 1)
	return errs[0]
}

func (ss *scanSuite) TestIterVals_rejectsUnscannableTypes() {
	for want, err := range map[string]error{
		"goqu: IterVals cannot return sql.RawBytes, which is only valid until the next row is read " +
			"or the rows are closed; use []byte": ss.iterValsErr[sql.RawBytes](),
		"goqu: IterVals cannot scan into []string; use []byte or a sql.Scanner":               ss.iterValsErr[[]string](),
		"goqu: IterVals cannot scan into goqu_test.scanTestBlob; use *goqu_test.scanTestBlob": ss.iterValsErr[scanTestBlob](),
	} {
		ss.EqualError(err, want)
		ss.True(goqu.IsUnsupportedScanType(err))
	}
}

func (ss *scanSuite) TestIterVals_fromExecutor() {
	ss.mock.ExpectQuery(`INSERT INTO "items" ("name") VALUES ('Test1'), ('Test2') RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2"))

	ds := ss.db.Insert("items").Rows(goqu.Record{"name": "Test1"}, goqu.Record{"name": "Test2"}).Returning("id")
	ids, errs := collect(ds.Executor().IterVals[int64](context.Background()))
	ss.Empty(errs)
	ss.Equal([]int64{1, 2}, ids)
}

func (ss *scanSuite) TestIterVals_withoutDatabase() {
	_, errs := collect(goqu.From("items").Select("id").IterVals[int64](context.Background()))
	ss.Require().Len(errs, 1)
	ss.True(goqu.IsNoDatabase(errs[0]))
}
