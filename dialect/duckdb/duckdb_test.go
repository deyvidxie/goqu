package duckdb_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/marcboeker/go-duckdb"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/dialect/duckdb"
	"github.com/doug-martin/goqu/v9/dialect/mysql"

	"github.com/stretchr/testify/suite"
)

const (
	dropTable   = "DROP TABLE IF EXISTS \"entry\"; DROP TABLE IF EXISTS \"entry_unindexed\"; DROP SEQUENCE IF EXISTS \"entry_id_seq\";"
	createTable = "CREATE SEQUENCE \"entry_id_seq\"; CREATE TABLE \"entry\" (" +
		"\"id\" INTEGER PRIMARY KEY DEFAULT nextval('entry_id_seq')," +
		"\"int\" INT NOT NULL ," +
		"\"float\" DOUBLE NOT NULL ," +
		"\"string\" VARCHAR(255) NOT NULL ," +
		"\"time\" DATETIME NOT NULL ," +
		"\"bool\" BOOLEAN NOT NULL ," +
		"\"bytes\" BLOB NOT NULL" +
		");"
	insertDefaultRecords = "INSERT INTO \"entry\" (\"int\", \"float\", \"string\", \"time\", \"bool\", \"bytes\") VALUES" +
		"(0, 0.000000, '0.000000', '2015-02-22T18:19:55.000000000-00:00', TRUE, '0.000000')," +
		"(1, 0.100000, '0.100000', '2015-02-22T19:19:55.000000000-00:00', FALSE, '0.100000')," +
		"(2, 0.200000, '0.200000', '2015-02-22T20:19:55.000000000-00:00', TRUE, '0.200000')," +
		"(3, 0.300000, '0.300000', '2015-02-22T21:19:55.000000000-00:00', FALSE, '0.300000')," +
		"(4, 0.400000, '0.400000', '2015-02-22T22:19:55.000000000-00:00', TRUE, '0.400000')," +
		"(5, 0.500000, '0.500000', '2015-02-22T23:19:55.000000000-00:00', FALSE, '0.500000')," +
		"(6, 0.600000, '0.600000', '2015-02-23T00:19:55.000000000-00:00', TRUE, '0.600000')," +
		"(7, 0.700000, '0.700000', '2015-02-23T01:19:55.000000000-00:00', FALSE, '0.700000')," +
		"(8, 0.800000, '0.800000', '2015-02-23T02:19:55.000000000-00:00', TRUE, '0.800000')," +
		"(9, 0.900000, '0.900000', '2015-02-23T03:19:55.000000000-00:00', FALSE, '0.900000');"
)

type (
	duckdbSuite struct {
		suite.Suite
		db *goqu.Database
	}
	entry struct {
		ID     uint32    `db:"id" goqu:"skipinsert,skipupdate"`
		Int    int       `db:"int"`
		Float  float64   `db:"float"`
		String string    `db:"string"`
		Time   time.Time `db:"time"`
		Bool   bool      `db:"bool"`
		Bytes  []byte    `db:"bytes"`
	}
	entryTestCase struct {
		ds    *goqu.SelectDataset
		len   int
		check func(entry entry, index int)
		err   string
	}
)

func (st *duckdbSuite) SetupSuite() {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		panic(err.Error())
	}
	st.db = goqu.New("duckdb", db)
}

func (st *duckdbSuite) assertSQL(cases ...sqlTestCase) {
	for i, c := range cases {
		actualSQL, actualArgs, err := c.ds.ToSQL()
		if c.err == "" {
			st.NoError(err, "test case %d failed", i)
		} else {
			st.EqualError(err, c.err, "test case %d failed", i)
		}
		st.Equal(c.sql, actualSQL, "test case %d failed", i)
		if c.isPrepared && c.args != nil || len(c.args) > 0 {
			st.Equal(c.args, actualArgs, "test case %d failed", i)
		} else {
			st.Empty(actualArgs, "test case %d failed", i)
		}
	}
}

func (st *duckdbSuite) assertEntries(cases ...entryTestCase) {
	for i, c := range cases {
		var entries []entry
		err := c.ds.ScanStructs(&entries)
		if c.err == "" {
			st.NoError(err, "test case %d failed", i)
		} else {
			st.EqualError(err, c.err, "test case %d failed", i)
		}
		st.Len(entries, c.len)
		for index, entry := range entries {
			c.check(entry, index)
		}
	}
}

func (st *duckdbSuite) SetupTest() {
	if _, err := st.db.Exec(dropTable); err != nil {
		panic(err)
	}
	if _, err := st.db.Exec(createTable); err != nil {
		panic(err)
	}
	if _, err := st.db.Exec(insertDefaultRecords); err != nil {
		panic(err)
	}
}

func (st *duckdbSuite) TestTimeFunctions() {
	for _, prepared := range []bool{false, true} {
		ds := st.db.From("entry").Prepared(prepared)
		count, err := ds.Where(goqu.C("time").Lt(goqu.Now(6).Sub(goqu.Interval(1, "year")))).Count()
		st.Require().NoError(err)
		st.Equal(int64(10), count)

		oneSecond := goqu.Func("epoch_us", goqu.Now().Add(goqu.Interval(1, "second"))).Sub(goqu.Func("epoch_us", goqu.Now()))
		count, err = ds.Where(oneSecond.Eq(1000000)).Count()
		st.Require().NoError(err)
		st.Equal(int64(10), count)

		quarter := goqu.L("make_timestamp(epoch_us(now())) + INTERVAL 3 MONTH")
		count, err = ds.Where(goqu.UTCTimestamp(6).Add(goqu.Interval(1, "quarter")).Eq(quarter)).Count()
		st.Require().NoError(err)
		st.Equal(int64(10), count)

		count, err = ds.Where(goqu.C("time").Lt(goqu.Now(3).Sub(goqu.Interval(1, "day")))).Count()
		st.Require().NoError(err)
		st.Equal(int64(10), count)

		count, err = ds.Where(goqu.C("time").Lt(goqu.UTCTimestamp(0))).Count()
		st.Require().NoError(err)
		st.Equal(int64(10), count)

		_, err = ds.Where(goqu.C("time").Lt(goqu.Now(2))).Count()
		st.EqualError(err, "goqu: dialect does not support fsp 2, want one of 0, 3, 6 [dialect=duckdb]")

		_, err = ds.Where(goqu.C("time").Lt(goqu.Now().Sub(goqu.Interval(1.5, goqu.SecondInterval)))).Count()
		st.EqualError(err, "goqu: Interval count must be an integer or an expression, got float64 1.5 for SECOND")
	}
}

func (st *duckdbSuite) TestSelectSQL() {
	ds := st.db.From("entry")
	st.assertSQL(
		sqlTestCase{
			ds:  ds.Select("id", "float", "string", "time", "bool"),
			sql: "SELECT \"id\", \"float\", \"string\", \"time\", \"bool\" FROM \"entry\"",
		},
		sqlTestCase{ds: ds.Where(goqu.C("int").Eq(10)), sql: "SELECT * FROM \"entry\" WHERE (\"int\" = 10)"},
		sqlTestCase{
			ds:   ds.Prepared(true).Where(goqu.L("? = ?", goqu.C("int"), 10)),
			sql:  "SELECT * FROM \"entry\" WHERE \"int\" = ?",
			args: []interface{}{int64(10)},
		},
	)
}

func (st *duckdbSuite) TestQuoteCharacterInIdentifier() {
	ds := st.db.From("entry")

	var v string
	found, err := ds.Select(goqu.L("'x'").As("a\"b")).ScanVal(&v)
	st.NoError(err)
	st.True(found)
	st.Equal("x", v)

	_, err = ds.Where(goqu.C("int\" > -1 OR \"int").Eq(-5)).Count()
	st.Error(err)
}

func (st *duckdbSuite) TestGreatestAndLeast() {
	ds := st.db.From("entry").Order(goqu.C("id").Asc())
	var greatest []int
	st.NoError(ds.Select(goqu.GREATEST(goqu.C("int"), 5)).ScanVals(&greatest))
	st.Equal([]int{5, 5, 5, 5, 5, 5, 6, 7, 8, 9}, greatest)

	var least []int
	st.NoError(ds.Select(goqu.LEAST(goqu.C("int"), 5)).ScanVals(&least))
	st.Equal([]int{0, 1, 2, 3, 4, 5, 5, 5, 5, 5}, least)
}

func (st *duckdbSuite) TestForUpdateSkipLockedRuns() {
	var entries []entry
	st.NoError(st.db.From("entry").ForUpdate(goqu.SkipLocked).ScanStructs(&entries))
	st.Len(entries, 10)
}

func (st *duckdbSuite) TestILike() {
	ds := st.db.From("entry")
	mixed := entry{Int: 10, Float: 1.0, String: "Mixed Case", Time: time.Now(), Bool: true, Bytes: []byte("x")}
	_, err := ds.Insert().Rows(mixed).Executor().Exec()
	st.Require().NoError(err)

	count, err := ds.Where(goqu.C("string").ILike("mixed%")).Count()
	st.NoError(err)
	st.Equal(int64(1), count)

	count, err = ds.Where(goqu.C("string").Like("mixed%")).Count()
	st.NoError(err)
	st.Zero(count)

	count, err = ds.Where(goqu.C("string").NotILike("mixed%")).Count()
	st.NoError(err)
	st.Equal(int64(10), count)
}

func (st *duckdbSuite) TestILikeEscapesLiteralPercent() {
	ds := st.db.From("entry")
	_, err := ds.Insert().Rows(
		entry{Int: 10, Float: 1.0, String: "50% off", Time: time.Now(), Bool: true, Bytes: []byte("x")},
		entry{Int: 11, Float: 1.0, String: "500 off", Time: time.Now(), Bool: true, Bytes: []byte("x")},
	).Executor().Exec()
	st.Require().NoError(err)

	count, err := ds.Where(goqu.C("string").ILike("50\\%%")).Count()
	st.NoError(err)
	st.Equal(int64(1), count)

	count, err = ds.Where(goqu.C("string").NotILike("50\\%%")).Count()
	st.NoError(err)
	st.Equal(int64(11), count)
}

func (st *duckdbSuite) TestDistinctOn() {
	var ints []int
	st.NoError(st.db.From("entry").Select("int").Distinct("bool").Order(goqu.C("bool").Asc(), goqu.C("int").Asc()).ScanVals(&ints))
	st.Equal([]int{1, 0}, ints)
}

func (st *duckdbSuite) TestWindowFunctionWithQualify() {
	var ints []int
	err := st.db.From("entry").
		Select("int").
		Window(goqu.W("w").PartitionBy("bool").OrderBy(goqu.C("int").Desc())).
		Qualify(goqu.ROW_NUMBER().OverName(goqu.I("w")).Eq(1)).
		Order(goqu.C("int").Asc()).
		ScanVals(&ints)
	st.NoError(err)
	st.Equal([]int{8, 9}, ints)
}

func (st *duckdbSuite) TestLateralJoin() {
	maxInt := goqu.From(goqu.T("entry").As("e2")).
		Select(goqu.MAX("int").As("max_int")).
		Where(goqu.I("e2.bool").Eq(goqu.I("e.bool"))).
		As("m")
	var rows []struct {
		Int    int `db:"int"`
		MaxInt int `db:"max_int"`
	}
	err := st.db.From(goqu.T("entry").As("e")).
		Select("e.int", "m.max_int").
		Join(goqu.Lateral(maxInt), goqu.On(goqu.V(true))).
		Where(goqu.I("e.int").In(0, 1)).
		Order(goqu.I("e.int").Asc()).
		ScanStructs(&rows)
	st.NoError(err)
	st.Len(rows, 2)
	st.Equal(8, rows[0].MaxInt)
	st.Equal(9, rows[1].MaxInt)
}

func (st *duckdbSuite) TestUpdateFrom() {
	_, err := st.db.Exec(`CREATE TABLE "labels" ("int" INT, "label" VARCHAR)`)
	st.Require().NoError(err)
	defer func() {
		_, err := st.db.Exec(`DROP TABLE "labels"`)
		st.NoError(err)
	}()
	_, err = st.db.Exec(`INSERT INTO "labels" VALUES (1, 'one')`)
	st.Require().NoError(err)

	_, err = st.db.Update("entry").
		Set(goqu.Record{"string": goqu.I("labels.label")}).
		From("labels").
		Where(goqu.I("entry.int").Eq(goqu.I("labels.int"))).
		Executor().Exec()
	st.NoError(err)

	var s string
	found, err := st.db.From("entry").Select("string").Where(goqu.C("int").Eq(1)).ScanVal(&s)
	st.NoError(err)
	st.True(found)
	st.Equal("one", s)
}

func (st *duckdbSuite) TestInsertOnConflictUpdateWhere() {
	var id int64
	found, err := st.db.From("entry").Select("id").Where(goqu.C("int").Eq(1)).ScanVal(&id)
	st.Require().NoError(err)
	st.Require().True(found)
	upsert := func(where goqu.Expression) {
		_, err := st.db.Insert("entry").
			Cols("id", "int", "float", "string", "time", "bool", "bytes").
			Vals(goqu.Vals{id, 1, 0.1, "x", time.Now(), false, []byte("x")}).
			OnConflict(goqu.DoUpdate("id", goqu.Record{"string": "updated"}).Where(where)).
			Executor().Exec()
		st.NoError(err)
	}

	upsert(goqu.I("entry.int").Eq(-1))
	var s string
	_, err = st.db.From("entry").Select("string").Where(goqu.C("id").Eq(id)).ScanVal(&s)
	st.NoError(err)
	st.Equal("0.100000", s)

	upsert(goqu.I("entry.int").Eq(1))
	_, err = st.db.From("entry").Select("string").Where(goqu.C("id").Eq(id)).ScanVal(&s)
	st.NoError(err)
	st.Equal("updated", s)
}

func (st *duckdbSuite) TestBitwiseInversion() {
	var v int64
	found, err := st.db.From("entry").Select(goqu.C("int").BitwiseInversion()).Where(goqu.C("int").Eq(5)).ScanVal(&v)
	st.NoError(err)
	st.True(found)
	st.Equal(int64(-6), v)
}

func (st *duckdbSuite) TestInsertDefaultValues() {
	_, err := st.db.Exec(`CREATE TABLE "defaults" ("id" INTEGER DEFAULT 1, "name" VARCHAR DEFAULT 'x')`)
	st.Require().NoError(err)
	defer func() {
		_, err := st.db.Exec(`DROP TABLE "defaults"`)
		st.NoError(err)
	}()

	_, err = st.db.Insert("defaults").Executor().Exec()
	st.NoError(err)

	var name string
	found, err := st.db.From("defaults").Select("name").Where(goqu.C("id").Eq(1)).ScanVal(&name)
	st.NoError(err)
	st.True(found)
	st.Equal("x", name)
}

func (st *duckdbSuite) TestCompoundQueries() {
	ds1 := st.db.From("entry").Select("int").Where(goqu.C("int").Gt(0))
	ds2 := st.db.From("entry").Select("int").Where(goqu.C("int").Gt(5))

	var ids []int64
	err := ds1.Union(ds2).ScanVals(&ids)
	st.NoError(err)
	st.ElementsMatch([]int64{1, 2, 3, 4, 5, 6, 7, 8, 9}, ids)

	ids = ids[0:0]
	err = ds1.UnionAll(ds2).ScanVals(&ids)
	st.NoError(err)
	st.ElementsMatch([]int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 6, 7, 8, 9}, ids)

	ids = ids[0:0]
	err = ds1.Intersect(ds2).ScanVals(&ids)
	st.NoError(err)
	st.ElementsMatch([]int64{6, 7, 8, 9}, ids)
}

func (st *duckdbSuite) TestQuery() {
	ds := st.db.From("entry")
	floatVal := float64(0)
	baseDate, err := time.Parse(duckdb.DialectOptions().TimeFormat, "2015-02-22T18:19:55.000000000-00:00")
	st.NoError(err)
	st.assertEntries(
		entryTestCase{ds: ds.Order(goqu.C("id").Asc()), len: 10, check: func(entry entry, index int) {
			f := fmt.Sprintf("%f", floatVal)
			st.EqualValues(index+1, entry.ID)
			st.Equal(index, entry.Int)
			st.Equal(f, fmt.Sprintf("%f", entry.Float))
			st.Equal(f, entry.String)
			st.Equal([]byte(f), entry.Bytes)
			st.Equal(index%2 == 0, entry.Bool)
			st.Equal(baseDate.Add(time.Duration(index)*time.Hour).Unix(), entry.Time.Unix())
			floatVal += float64(0.1)
		}},
		entryTestCase{ds: ds.Where(goqu.C("bool").IsTrue()).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Bool)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gt(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int > 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gte(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int >= 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lt(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int < 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lte(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int <= 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Between(goqu.Range(3, 6))).Order(goqu.C("id").Asc()), len: 4, check: func(entry entry, _ int) {
			st.True(entry.Int >= 3)
			st.True(entry.Int <= 6)
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Eq("0.100000")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			st.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Like("0.1%")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			st.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").NotLike("0.1%")).Order(goqu.C("id").Asc()), len: 9, check: func(entry entry, _ int) {
			st.NotEqual(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").IsNull()).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			st.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").In([]int{})).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			st.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.Ex{"int": goqu.Op{"notIn": []int{}}}).Order(goqu.C("id").Asc()), len: 10, check: func(_ entry, _ int) {}},
	)
}

func (st *duckdbSuite) TestQuery_Prepared() {
	ds := st.db.From("entry").Prepared(true)
	floatVal := float64(0)
	baseDate, err := time.Parse(duckdb.DialectOptions().TimeFormat, "2015-02-22T18:19:55.000000000-00:00")
	st.NoError(err)
	st.assertEntries(
		entryTestCase{ds: ds.Order(goqu.C("id").Asc()), len: 10, check: func(entry entry, index int) {
			f := fmt.Sprintf("%f", floatVal)
			st.EqualValues(index+1, entry.ID)
			st.Equal(index, entry.Int)
			st.Equal(f, fmt.Sprintf("%f", entry.Float))
			st.Equal(f, entry.String)
			st.Equal([]byte(f), entry.Bytes)
			st.Equal(index%2 == 0, entry.Bool)
			st.Equal(baseDate.Add(time.Duration(index)*time.Hour).Unix(), entry.Time.Unix())
			floatVal += float64(0.1)
		}},
		entryTestCase{ds: ds.Where(goqu.C("bool").IsTrue()).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Bool)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gt(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int > 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gte(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int >= 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lt(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int < 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lte(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			st.True(entry.Int <= 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Between(goqu.Range(3, 6))).Order(goqu.C("id").Asc()), len: 4, check: func(entry entry, _ int) {
			st.True(entry.Int >= 3)
			st.True(entry.Int <= 6)
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Eq("0.100000")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			st.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Like("0.1%")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			st.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").NotLike("0.1%")).Order(goqu.C("id").Asc()), len: 9, check: func(entry entry, _ int) {
			st.NotEqual(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").IsNull()).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			st.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").In([]int{})).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			st.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.Ex{"int": goqu.Op{"notIn": []int{}}}).Order(goqu.C("id").Asc()), len: 10, check: func(_ entry, _ int) {}},
	)
}

func (st *duckdbSuite) TestQuery_ValueExpressions() {
	type wrappedEntry struct {
		entry
		BoolValue bool `db:"bool_value"`
	}
	expectedDate, err := time.Parse("2006-01-02T15:04:05.000000000-00:00", "2015-02-22T19:19:55.000000000-00:00")
	st.NoError(err)
	ds := st.db.From("entry").Select(goqu.Star(), goqu.V(true).As("bool_value")).Where(goqu.Ex{"int": 1})
	var we wrappedEntry
	found, err := ds.ScanStruct(&we)
	st.NoError(err)
	st.True(found)
	st.Equal(we, wrappedEntry{
		entry{2, 1, 0.100000, "0.100000", expectedDate, false, []byte("0.100000")},
		true,
	})
}

func (st *duckdbSuite) TestCount() {
	ds := st.db.From("entry")
	count, err := ds.Count()
	st.NoError(err)
	st.Equal(int64(10), count)
	count, err = ds.Where(goqu.C("int").Gt(4)).Count()
	st.NoError(err)
	st.Equal(int64(5), count)
	count, err = ds.Where(goqu.C("int").Gte(4)).Count()
	st.NoError(err)
	st.Equal(int64(6), count)
	count, err = ds.Where(goqu.C("string").Like("0.1%")).Count()
	st.NoError(err)
	st.Equal(int64(1), count)
	count, err = ds.Where(goqu.C("string").IsNull()).Count()
	st.NoError(err)
	st.Equal(int64(0), count)
}

func (st *duckdbSuite) TestInsert() {
	ds := st.db.From("entry")
	now := time.Now()
	e := entry{Int: 10, Float: 1.000000, String: "1.000000", Time: now, Bool: true, Bytes: []byte("1.000000")}
	_, err := ds.Insert().Rows(e).Executor().Exec()
	st.NoError(err)

	var insertedEntry entry
	found, err := ds.Where(goqu.C("int").Eq(10)).ScanStruct(&insertedEntry)
	st.NoError(err)
	st.True(found)
	st.Equal(uint32(11), insertedEntry.ID)

	entries := []entry{
		{Int: 11, Float: 1.100000, String: "1.100000", Time: now, Bool: false, Bytes: []byte("1.100000")},
		{Int: 12, Float: 1.200000, String: "1.200000", Time: now, Bool: true, Bytes: []byte("1.200000")},
		{Int: 13, Float: 1.300000, String: "1.300000", Time: now, Bool: false, Bytes: []byte("1.300000")},
		{Int: 14, Float: 1.400000, String: "1.400000", Time: now, Bool: true, Bytes: []byte("1.400000")},
		{Int: 14, Float: 1.400000, String: `abc'd"e"f\\gh\n\ri\x00`, Time: now, Bool: true, Bytes: []byte("1.400000")},
	}
	_, err = ds.Insert().Rows(entries).Executor().Exec()
	st.NoError(err)

	var newEntries []entry
	st.NoError(ds.Where(goqu.C("int").In([]uint32{11, 12, 13, 14})).Order(goqu.C("id").Asc()).ScanStructs(&newEntries))
	st.Len(newEntries, len(entries))
	for i, e := range newEntries {
		st.Equal(entries[i].Int, e.Int)
		st.Equal(entries[i].Float, e.Float)
		st.Equal(entries[i].String, e.String)
		st.Equal(entries[i].Time.UTC().Format(mysql.DialectOptions().TimeFormat), e.Time.Format(mysql.DialectOptions().TimeFormat))
		st.Equal(entries[i].Bool, e.Bool)
		st.Equal(entries[i].Bytes, e.Bytes)
	}

	_, err = ds.Insert().Rows(
		entry{Int: 15, Float: 1.500000, String: "1.500000", Time: now, Bool: false, Bytes: []byte("1.500000")},
		entry{Int: 16, Float: 1.600000, String: "1.600000", Time: now, Bool: true, Bytes: []byte("1.600000")},
		entry{Int: 17, Float: 1.700000, String: "1.700000", Time: now, Bool: false, Bytes: []byte("1.700000")},
		entry{Int: 18, Float: 1.800000, String: "1.800000", Time: now, Bool: true, Bytes: []byte("1.800000")},
		entry{Int: 18, Float: 1.800000, String: `abc'd"e"f\\gh\n\ri\x00`, Time: now, Bool: true, Bytes: []byte("1.800000")},
	).Executor().Exec()
	st.NoError(err)

	newEntries = newEntries[0:0]
	st.NoError(ds.Where(goqu.C("int").In([]uint32{15, 16, 17, 18})).ScanStructs(&newEntries))
	st.Len(newEntries, 5)
}

func (st *duckdbSuite) TestInsert_returning() {
	ds := st.db.From("entry")
	now := time.Now()
	e := entry{Int: 10, Float: 1.000000, String: "1.000000", Time: now, Bool: true, Bytes: []byte("1.000000")}
	found, err := ds.Insert().Rows(e).Returning(goqu.Star()).Executor().ScanStruct(&e)
	st.NoError(err)
	st.True(found)
	st.Equal(uint32(11), e.ID)
}

func (st *duckdbSuite) TestUpdate() {
	ds := st.db.From("entry")
	var e entry
	found, err := ds.Where(goqu.C("int").Eq(9)).Select("id").ScanStruct(&e)
	st.NoError(err)
	st.True(found)
	e.Int = 11
	_, err = ds.Where(goqu.C("id").Eq(e.ID)).Update().Set(e).Executor().Exec()
	st.NoError(err)

	count, err := ds.Where(goqu.C("int").Eq(11)).Count()
	st.NoError(err)
	st.Equal(int64(1), count)
}

func (st *duckdbSuite) TestUpdateReturning() {
	// DuckDB 1.1.1 rejects UPDATE ... RETURNING on tables with a primary key ("Duplicate key" constraint error).
	_, err := st.db.Exec("CREATE OR REPLACE TABLE \"entry_unindexed\" AS SELECT * FROM \"entry\"")
	st.Require().NoError(err)
	ds := st.db.From("entry_unindexed")
	var id uint32
	found, err := ds.
		Where(goqu.C("id").Eq(1)).
		Update().
		Set(goqu.Record{"int": 11}).
		Returning("id").
		Executor().ScanVal(&id)
	st.NoError(err)
	st.True(found)
	st.Equal(uint32(1), id)

	var updated int
	found, err = ds.Select("int").Where(goqu.C("id").Eq(1)).ScanVal(&updated)
	st.NoError(err)
	st.True(found)
	st.Equal(11, updated)
}

func (st *duckdbSuite) TestDelete() {
	ds := st.db.From("entry")
	var e entry
	found, err := ds.Where(goqu.C("int").Eq(9)).Select("id").ScanStruct(&e)
	st.NoError(err)
	st.True(found)
	_, err = ds.Where(goqu.C("id").Eq(e.ID)).Delete().Executor().Exec()
	st.NoError(err)

	count, err := ds.Count()
	st.NoError(err)
	st.Equal(int64(9), count)

	var id uint32
	found, err = ds.Where(goqu.C("id").Eq(e.ID)).ScanVal(&id)
	st.NoError(err)
	st.False(found)

	e = entry{}
	found, err = ds.Where(goqu.C("int").Eq(8)).Select("id").ScanStruct(&e)
	st.NoError(err)
	st.True(found)
	st.NotZero(e.ID)

	id = 0
	found, err = ds.Where(goqu.C("id").Eq(e.ID)).Delete().Returning("id").Executor().ScanVal(&id)
	st.NoError(err)
	st.True(found)
	st.Equal(e.ID, id)

	count, err = ds.Count()
	st.NoError(err)
	st.Equal(int64(8), count)
}

func (st *duckdbSuite) TestDeleteWithLimitIsRejected() {
	_, err := st.db.From("entry").Where(goqu.C("int").Lt(5)).Limit(1).Delete().Executor().Exec()
	st.ErrorContains(err, `syntax error at or near "LIMIT"`)
	count, err := st.db.From("entry").Count()
	st.NoError(err)
	st.Equal(int64(10), count)
}

func (st *duckdbSuite) TestDeleteWithOrderIsRejected() {
	_, err := st.db.From("entry").Where(goqu.C("int").Lt(5)).Order(goqu.C("id").Asc()).Delete().Executor().Exec()
	st.EqualError(err, "goqu: dialect does not support ORDER BY on DELETE [dialect=duckdb]")
	count, err := st.db.From("entry").Count()
	st.NoError(err)
	st.Equal(int64(10), count)
}

func (st *duckdbSuite) TestUpdateWithLimitIsRejected() {
	_, err := st.db.From("entry").Where(goqu.C("int").Lt(5)).Limit(1).Update().Set(goqu.Record{"int": 99}).Executor().Exec()
	st.ErrorContains(err, `syntax error at or near "LIMIT"`)
	count, err := st.db.From("entry").Where(goqu.C("int").Eq(99)).Count()
	st.NoError(err)
	st.Zero(count)
}

func (st *duckdbSuite) TestUpdateWithOrderIsRejected() {
	_, err := st.db.From("entry").Where(goqu.C("int").Lt(5)).Order(goqu.C("id").Asc()).Update().Set(goqu.Record{"int": 99}).Executor().Exec()
	st.ErrorContains(err, `syntax error at or near "ORDER"`)
	count, err := st.db.From("entry").Where(goqu.C("int").Eq(99)).Count()
	st.NoError(err)
	st.Zero(count)
}

func (st *duckdbSuite) TestInsert_OnConflictExcluded() {
	ds := st.db.From("entry")
	existing := goqu.T("entry")
	upsert := func(float float64, s string) {
		_, err := ds.Insert().
			Rows(goqu.Record{"id": 2, "int": 1, "float": float, "string": s, "time": time.Now(), "bool": false, "bytes": []byte(s)}).
			OnConflict(goqu.DoUpdate("id", goqu.Record{
				"string": goqu.Excluded("string"),
				"float":  existing.Col("float").Add(goqu.Excluded("float")),
			}).Where(existing.Col("float").Lt(goqu.Excluded("float")))).
			Executor().Exec()
		st.Require().NoError(err)
	}
	upsert(10, "incoming")
	upsert(0.05, "skipped")

	var actual entry
	_, err := ds.Where(goqu.C("id").Eq(2)).ScanStruct(&actual)
	st.Require().NoError(err)
	st.Equal("incoming", actual.String)
	st.InDelta(10.1, actual.Float, 0.001)
}

func (st *duckdbSuite) TestInsert_OnConflict() {
	ds := st.db.From("entry")
	now := time.Now()

	e := entry{Int: 11, Float: 1.100000, String: "1.100000", Time: now, Bool: false, Bytes: []byte("1.100000")}
	_, err := ds.Insert().Rows(e).OnConflict(goqu.DoNothing()).Executor().Exec()
	st.NoError(err)

	var entryActual entry
	_, err = ds.Where(goqu.C("id").Eq(11)).ScanStruct(&entryActual)
	st.NoError(err)
	st.Equal("1.100000", entryActual.String)

	_, err = ds.Insert().Rows(
		goqu.Record{
			"id":     11,
			"int":    99999999,
			"float":  "0.99999999",
			"string": "99999999",
			"time":   now,
			"bool":   true,
			"bytes":  []byte("0.99999999"),
		},
	).OnConflict(goqu.DoNothing()).Executor().Exec()
	st.NoError(err)

	_, err = ds.Where(goqu.C("id").Eq(11)).ScanStruct(&entryActual)
	st.NoError(err)
	st.Equal("1.100000", entryActual.String)

	_, err = ds.Insert().Rows(
		goqu.Record{
			"id":     11,
			"int":    11,
			"float":  "1.100000",
			"string": "1.100000",
			"time":   now,
			"bool":   true,
			"bytes":  []byte("1.100000"),
		},
	).OnConflict(goqu.DoUpdate("id", goqu.Record{"string": "upsert"})).Executor().Exec()
	st.NoError(err)

	_, err = ds.Where(goqu.C("id").Eq(11)).ScanStruct(&entryActual)
	st.NoError(err)
	st.Equal("upsert", entryActual.String)
}

func TestDuckdbSuite(t *testing.T) {
	suite.Run(t, new(duckdbSuite))
}
