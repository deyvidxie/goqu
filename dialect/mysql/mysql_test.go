package mysql_test

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/suite"
)

const (
	dropTable   = "DROP TABLE IF EXISTS `entry`;"
	createTable = "CREATE  TABLE `entry` (" +
		"`id` INT NOT NULL AUTO_INCREMENT ," +
		"`int` INT NOT NULL UNIQUE," +
		"`float` FLOAT NOT NULL ," +
		"`string` VARCHAR(255) NOT NULL ," +
		"`time` DATETIME NOT NULL ," +
		"`bool` TINYINT NOT NULL ," +
		"`bytes` BLOB NOT NULL ," +
		"PRIMARY KEY (`id`) );"
	insertDefaultReords = "INSERT INTO `entry` (`int`, `float`, `string`, `time`, `bool`, `bytes`) VALUES" +
		"(0, 0.000000, '0.000000', '2015-02-22 18:19:55', TRUE,  '0.000000')," +
		"(1, 0.100000, '0.100000', '2015-02-22 19:19:55', FALSE, '0.100000')," +
		"(2, 0.200000, '0.200000', '2015-02-22 20:19:55', TRUE,  '0.200000')," +
		"(3, 0.300000, '0.300000', '2015-02-22 21:19:55', FALSE, '0.300000')," +
		"(4, 0.400000, '0.400000', '2015-02-22 22:19:55', TRUE,  '0.400000')," +
		"(5, 0.500000, '0.500000', '2015-02-22 23:19:55', FALSE, '0.500000')," +
		"(6, 0.600000, '0.600000', '2015-02-23 00:19:55', TRUE,  '0.600000')," +
		"(7, 0.700000, '0.700000', '2015-02-23 01:19:55', FALSE, '0.700000')," +
		"(8, 0.800000, '0.800000', '2015-02-23 02:19:55', TRUE,  '0.800000')," +
		"(9, 0.900000, '0.900000', '2015-02-23 03:19:55', FALSE, '0.900000');"
)

const defaultDBURI = "root@/goqumysql?parseTime=true"

type (
	mysqlTest struct {
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

func (mt *mysqlTest) SetupSuite() {
	dbURI := os.Getenv("MYSQL_URI")
	if dbURI == "" {
		dbURI = defaultDBURI
	}
	db, err := sql.Open("mysql", dbURI)
	if err != nil {
		panic(err.Error())
	}
	mt.db = goqu.New("mysql", db)
}

func (mt *mysqlTest) assertSQL(cases ...sqlTestCase) {
	for i, c := range cases {
		actualSQL, actualArgs, err := c.ds.ToSQL()
		if c.err == "" {
			mt.NoError(err, "test case %d failed", i)
		} else {
			mt.EqualError(err, c.err, "test case %d failed", i)
		}
		mt.Equal(c.sql, actualSQL, "test case %d failed", i)
		if c.isPrepared && c.args != nil || len(c.args) > 0 {
			mt.Equal(c.args, actualArgs, "test case %d failed", i)
		} else {
			mt.Empty(actualArgs, "test case %d failed", i)
		}
	}
}

func (mt *mysqlTest) assertEntries(cases ...entryTestCase) {
	for i, c := range cases {
		var entries []entry
		err := c.ds.ScanStructs(&entries)
		if c.err == "" {
			mt.NoError(err, "test case %d failed", i)
		} else {
			mt.EqualError(err, c.err, "test case %d failed", i)
		}
		mt.Len(entries, c.len)
		for index, entry := range entries {
			c.check(entry, index)
		}
	}
}

func (mt *mysqlTest) SetupTest() {
	if _, err := mt.db.Exec(dropTable); err != nil {
		panic(err)
	}
	if _, err := mt.db.Exec(createTable); err != nil {
		panic(err)
	}
	if _, err := mt.db.Exec(insertDefaultReords); err != nil {
		panic(err)
	}
}

func (mt *mysqlTest) SetupSubTest() {
	mt.SetupTest()
}

func (mt *mysqlTest) serverVersion() [3]int {
	var version string
	ok, err := mt.db.Select(goqu.Func("version")).ScanVal(&version)
	mt.Require().NoError(err)
	mt.Require().True(ok)
	var parts [3]int
	for i, field := range strings.SplitN(strings.SplitN(version, "-", 2)[0], ".", 3) {
		parts[i], err = strconv.Atoi(field)
		mt.Require().NoError(err)
	}
	return parts
}

func versionAtLeast(version, minimum [3]int) bool {
	for i := range version {
		if version[i] != minimum[i] {
			return version[i] > minimum[i]
		}
	}
	return true
}

func (mt *mysqlTest) TestTimeFunctionsInNonUTCSession() {
	tx, err := mt.db.Begin()
	mt.Require().NoError(err)
	defer func() { mt.NoError(tx.Rollback()) }()
	_, err = tx.Exec("SET time_zone = '+05:00'")
	mt.Require().NoError(err)

	var utc, local []time.Time
	mt.Require().NoError(tx.From("entry").Select(goqu.UTCTimestamp(6)).Limit(1).ScanVals(&utc))
	mt.Require().NoError(tx.From("entry").Select(goqu.Now(6)).Limit(1).ScanVals(&local))
	mt.Require().Len(utc, 1)
	mt.Require().Len(local, 1)
	mt.WithinDuration(time.Now().UTC(), utc[0], time.Minute)
	mt.WithinDuration(time.Now().UTC().Add(5*time.Hour), local[0], time.Minute)
}

func (mt *mysqlTest) TestTimeFunctions() {
	for _, prepared := range []bool{false, true} {
		ds := mt.db.From("entry").Prepared(prepared)
		count, err := ds.Where(goqu.C("time").Lt(goqu.Now(6).Sub(goqu.Interval(1, "year")))).Count()
		mt.Require().NoError(err)
		mt.Equal(int64(10), count)

		oneSecondLater := goqu.Func("DATE_ADD", goqu.Now(6), goqu.Interval(1, "second"))
		count, err = ds.Where(goqu.Func("TIMESTAMPDIFF", goqu.L("MICROSECOND"), goqu.Now(6), oneSecondLater).Eq(1000000)).Count()
		mt.Require().NoError(err)
		mt.Equal(int64(10), count)

		count, err = ds.Where(goqu.UTCTimestamp().Add(goqu.Interval(1, "quarter")).Eq(goqu.L("UTC_TIMESTAMP() + INTERVAL 3 MONTH"))).Count()
		mt.Require().NoError(err)
		mt.Equal(int64(10), count)

		var times []time.Time
		mt.Require().NoError(ds.Select(goqu.UTCTimestamp(6)).Limit(1).ScanVals(&times))
		mt.Require().Len(times, 1)
		mt.WithinDuration(time.Now().UTC(), times[0], time.Minute)
	}
}

func (mt *mysqlTest) TestToSQL() {
	ds := mt.db.From("entry")
	mt.assertSQL(
		sqlTestCase{ds: ds.Select("id", "float", "string", "time", "bool"), sql: "SELECT `id`, `float`, `string`, `time`, `bool` FROM `entry`"},
		sqlTestCase{ds: ds.Where(goqu.C("int").Eq(10)), sql: "SELECT * FROM `entry` WHERE (`int` = 10)"},
		sqlTestCase{
			ds:  ds.Prepared(true).Where(goqu.L("? = ?", goqu.C("int"), 10)),
			sql: "SELECT * FROM `entry` WHERE `int` = ?", args: []interface{}{int64(10)},
		},
	)
}

func (mt *mysqlTest) TestQuery() {
	ds := mt.db.From("entry")
	floatVal := float64(0)
	baseDate, err := time.Parse(
		"2006-01-02 15:04:05",
		"2015-02-22 18:19:55",
	)
	mt.NoError(err)
	mt.assertEntries(
		entryTestCase{ds: ds.Order(goqu.C("id").Asc()), len: 10, check: func(entry entry, index int) {
			f := fmt.Sprintf("%f", floatVal)
			mt.EqualValues(index+1, entry.ID)
			mt.Equal(index, entry.Int)
			mt.Equal(f, fmt.Sprintf("%f", entry.Float))
			mt.Equal(f, entry.String)
			mt.Equal([]byte(f), entry.Bytes)
			mt.Equal(index%2 == 0, entry.Bool)
			mt.Equal(baseDate.Add(time.Duration(index)*time.Hour).Unix(), entry.Time.Unix())
			floatVal += float64(0.1)
		}},
		entryTestCase{ds: ds.Where(goqu.C("bool").IsTrue()).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Bool)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gt(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int > 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gte(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int >= 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lt(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int < 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lte(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int <= 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Between(goqu.Range(3, 6))).Order(goqu.C("id").Asc()), len: 4, check: func(entry entry, _ int) {
			mt.True(entry.Int >= 3)
			mt.True(entry.Int <= 6)
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Eq("0.100000")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			mt.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Like("0.1%")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			mt.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").NotLike("0.1%")).Order(goqu.C("id").Asc()), len: 9, check: func(entry entry, _ int) {
			mt.NotEqual(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").IsNull()).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			mt.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").In([]int{})).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			mt.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.Ex{"int": goqu.Op{"notIn": []int{}}}).Order(goqu.C("id").Asc()), len: 10, check: func(_ entry, _ int) {}},
	)
}

func (mt *mysqlTest) TestQuery_Prepared() {
	ds := mt.db.From("entry").Prepared(true)
	floatVal := float64(0)
	baseDate, err := time.Parse(
		"2006-01-02 15:04:05",
		"2015-02-22 18:19:55",
	)
	mt.NoError(err)
	mt.assertEntries(
		entryTestCase{ds: ds.Order(goqu.C("id").Asc()), len: 10, check: func(entry entry, index int) {
			f := fmt.Sprintf("%f", floatVal)
			mt.EqualValues(index+1, entry.ID)
			mt.Equal(index, entry.Int)
			mt.Equal(f, fmt.Sprintf("%f", entry.Float))
			mt.Equal(f, entry.String)
			mt.Equal([]byte(f), entry.Bytes)
			mt.Equal(index%2 == 0, entry.Bool)
			mt.Equal(baseDate.Add(time.Duration(index)*time.Hour).Unix(), entry.Time.Unix())
			floatVal += float64(0.1)
		}},
		entryTestCase{ds: ds.Where(goqu.C("bool").IsTrue()).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Bool)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gt(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int > 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Gte(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int >= 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lt(5)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int < 5)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Lte(4)).Order(goqu.C("id").Asc()), len: 5, check: func(entry entry, _ int) {
			mt.True(entry.Int <= 4)
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").Between(goqu.Range(3, 6))).Order(goqu.C("id").Asc()), len: 4, check: func(entry entry, _ int) {
			mt.True(entry.Int >= 3)
			mt.True(entry.Int <= 6)
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Eq("0.100000")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			mt.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").Like("0.1%")).Order(goqu.C("id").Asc()), len: 1, check: func(entry entry, _ int) {
			mt.Equal(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").NotLike("0.1%")).Order(goqu.C("id").Asc()), len: 9, check: func(entry entry, _ int) {
			mt.NotEqual(entry.String, "0.100000")
		}},
		entryTestCase{ds: ds.Where(goqu.C("string").IsNull()).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			mt.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.C("int").In([]int{})).Order(goqu.C("id").Asc()), len: 0, check: func(_ entry, _ int) {
			mt.Fail("Should not have returned any records")
		}},
		entryTestCase{ds: ds.Where(goqu.Ex{"int": goqu.Op{"notIn": []int{}}}).Order(goqu.C("id").Asc()), len: 10, check: func(_ entry, _ int) {}},
	)
}

func (mt *mysqlTest) TestQuery_ValueExpressions() {
	type wrappedEntry struct {
		entry
		BoolValue bool `db:"bool_value"`
	}
	expectedDate, err := time.Parse("2006-01-02 15:04:05", "2015-02-22 19:19:55")
	mt.NoError(err)
	ds := mt.db.From("entry").Select(goqu.Star(), goqu.V(true).As("bool_value")).Where(goqu.Ex{"int": 1})
	var we wrappedEntry
	found, err := ds.ScanStruct(&we)
	mt.NoError(err)
	mt.True(found)
	mt.Equal(wrappedEntry{
		entry{2, 1, 0.100000, "0.100000", expectedDate, false, []byte("0.100000")},
		true,
	}, we)
}

func (mt *mysqlTest) TestCollate() {
	// a collation is applied to an expression: case sensitive and insensitive comparison of the same values
	ds := mt.db.From("entry")
	count, err := ds.Where(goqu.Collate(goqu.L("_utf8mb4'abc'"), "utf8mb4_bin").Eq("ABC")).Count()
	mt.NoError(err)
	mt.Equal(int64(0), count)
	count, err = ds.Where(goqu.Collate(goqu.L("_utf8mb4'abc'"), "utf8mb4_bin").Eq("abc")).Count()
	mt.NoError(err)
	mt.Equal(int64(10), count)
	count, err = ds.Where(goqu.Collate(goqu.L("_utf8mb4'abc'"), "utf8mb4_general_ci").Eq("ABC")).Count()
	mt.NoError(err)
	mt.Equal(int64(10), count)
}

func (mt *mysqlTest) TestInterpolatedStringLiteral() {
	ds := mt.db.From("entry")
	value := "it's a \\ path with \"quotes\"\n\r\x00\x1a"
	var v string
	found, err := ds.Select(goqu.V(value).As("v")).ScanVal(&v)
	mt.NoError(err)
	mt.True(found)
	mt.Equal(value, v)

	injection := "') OR 1=1 -- "
	count, err := ds.Where(goqu.C("string").Eq(injection)).Count()
	mt.NoError(err)
	mt.Equal(int64(0), count)

	tx, err := mt.db.Begin()
	mt.Require().NoError(err)
	defer func() { mt.NoError(tx.Rollback()) }()
	var sqlMode string
	mt.Require().NoError(tx.QueryRow("SELECT @@SESSION.sql_mode").Scan(&sqlMode))
	_, err = tx.Exec("SET SESSION sql_mode = 'NO_BACKSLASH_ESCAPES'")
	mt.Require().NoError(err)
	defer func() {
		_, err := tx.Exec("SET SESSION sql_mode = ?", sqlMode)
		mt.NoError(err)
	}()

	count, err = tx.From("entry").Where(goqu.C("string").Eq(injection)).Count()
	mt.NoError(err)
	mt.Equal(int64(0), count)
}

func (mt *mysqlTest) TestQuoteCharacterInIdentifier() {
	ds := mt.db.From("entry")

	var v string
	found, err := ds.Select(goqu.L("'x'").As("a`b")).ScanVal(&v)
	mt.NoError(err)
	mt.True(found)
	mt.Equal("x", v)

	_, err = ds.Where(goqu.C("int` > -1 OR `int").Eq(-5)).Count()
	mt.Error(err)
}

func (mt *mysqlTest) TestCount() {
	ds := mt.db.From("entry")
	count, err := ds.Count()
	mt.NoError(err)
	mt.Equal(int64(10), count)
	count, err = ds.Where(goqu.C("int").Gt(4)).Count()
	mt.NoError(err)
	mt.Equal(int64(5), count)
	count, err = ds.Where(goqu.C("int").Gte(4)).Count()
	mt.NoError(err)
	mt.Equal(int64(6), count)
	count, err = ds.Where(goqu.C("string").Like("0.1%")).Count()
	mt.NoError(err)
	mt.Equal(int64(1), count)
	count, err = ds.Where(goqu.C("string").IsNull()).Count()
	mt.NoError(err)
	mt.Equal(int64(0), count)
}

func (mt *mysqlTest) TestInsert() {
	ds := mt.db.From("entry")
	now := time.Now()
	e := entry{Int: 10, Float: 1.000000, String: "1.000000", Time: now, Bool: true, Bytes: []byte("1.000000")}
	_, err := ds.Insert().Rows(e).Executor().Exec()
	mt.NoError(err)

	var insertedEntry entry
	found, err := ds.Where(goqu.C("int").Eq(10)).ScanStruct(&insertedEntry)
	mt.NoError(err)
	mt.True(found)
	mt.True(insertedEntry.ID > 0)

	entries := []entry{
		{Int: 11, Float: 1.100000, String: "1.100000", Time: now, Bool: false, Bytes: []byte("1.100000")},
		{Int: 12, Float: 1.200000, String: "1.200000", Time: now, Bool: true, Bytes: []byte("1.200000")},
		{Int: 13, Float: 1.300000, String: "1.300000", Time: now, Bool: false, Bytes: []byte("1.300000")},
		{Int: 14, Float: 1.400000, String: "1.400000", Time: now, Bool: true, Bytes: []byte("1.400000")},
	}
	_, err = ds.Insert().Rows(entries).Executor().Exec()
	mt.NoError(err)

	var newEntries []entry
	mt.NoError(ds.Where(goqu.C("int").In([]uint32{11, 12, 13, 14})).ScanStructs(&newEntries))
	mt.Len(newEntries, 4)
	for i, e := range newEntries {
		mt.Equal(entries[i].Int, e.Int)
		mt.Equal(entries[i].Float, e.Float)
		mt.Equal(entries[i].String, e.String)
		mt.Equal(entries[i].Time.UTC().Format(mysql.DialectOptions().TimeFormat), e.Time.Format(mysql.DialectOptions().TimeFormat))
		mt.Equal(entries[i].Bool, e.Bool)
		mt.Equal(entries[i].Bytes, e.Bytes)
	}

	_, err = ds.Insert().Rows(
		entry{Int: 15, Float: 1.500000, String: "1.500000", Time: now, Bool: false, Bytes: []byte("1.500000")},
		entry{Int: 16, Float: 1.600000, String: "1.600000", Time: now, Bool: true, Bytes: []byte("1.600000")},
		entry{Int: 17, Float: 1.700000, String: "1.700000", Time: now, Bool: false, Bytes: []byte("1.700000")},
		entry{Int: 18, Float: 1.800000, String: "1.800000", Time: now, Bool: true, Bytes: []byte("1.800000")},
	).Executor().Exec()
	mt.NoError(err)

	newEntries = newEntries[0:0]
	mt.NoError(ds.Where(goqu.C("int").In([]uint32{15, 16, 17, 18})).ScanStructs(&newEntries))
	mt.Len(newEntries, 4)
}

func (mt *mysqlTest) TestInsertReturning() {
	ds := mt.db.From("entry")
	now := time.Now()
	e := entry{Int: 10, Float: 1.000000, String: "1.000000", Time: now, Bool: true, Bytes: []byte("1.000000")}
	_, err := ds.Insert().Rows(e).Returning(goqu.Star()).Executor().ScanStruct(&e)
	mt.Error(err)
}

func (mt *mysqlTest) TestJSONFunctions() {
	_, err := mt.db.Exec("CREATE TABLE `docs` (`id` INT NOT NULL, `doc` JSON NOT NULL)")
	mt.Require().NoError(err)
	defer func() {
		_, err := mt.db.Exec("DROP TABLE `docs`")
		mt.NoError(err)
	}()
	_, err = mt.db.Insert("docs").Rows(
		goqu.Record{"id": 1, "doc": `{"name": "bob", "n": 2, "tags": ["a\u0001b", "x"]}`},
		goqu.Record{"id": 2, "doc": `{"name": "sally", "n": 1, "tags": ["y"]}`},
	).Executor().Exec()
	mt.Require().NoError(err)

	for _, prepared := range []bool{false, true} {
		ds := mt.db.From("docs").Prepared(prepared)

		var ids []int64
		mt.Require().NoError(ds.Select("id").Where(goqu.JSONContains("doc", "a\x01b", "$.tags")).ScanVals(&ids))
		mt.Equal([]int64{1}, ids)

		var names []string
		mt.Require().NoError(ds.Select(goqu.JSONValue("doc", "$.name")).Order(goqu.JSONExtract("doc", "$.n").Asc()).ScanVals(&names))
		mt.Equal([]string{"sally", "bob"}, names)

		count, err := ds.Where(goqu.JSONContains("doc", map[string]interface{}{"name": "sally"})).Count()
		mt.Require().NoError(err)
		mt.Equal(int64(1), count)

		count, err = ds.Where(goqu.JSONContains("doc", []byte(`{"name": "bob"}`))).Count()
		mt.Require().NoError(err)
		mt.Equal(int64(1), count)

		count, err = ds.Where(goqu.JSONContains("doc", goqu.I("docs.doc"))).Count()
		mt.Require().NoError(err)
		mt.Equal(int64(2), count)
	}
}

func (mt *mysqlTest) TestMysql8NanoEqMatchesPreparedFractionalTime() {
	_, err := mt.db.Exec("CREATE TABLE `precise` (`t` DATETIME(6) NOT NULL)")
	mt.Require().NoError(err)
	defer func() {
		_, err := mt.db.Exec("DROP TABLE `precise`")
		mt.NoError(err)
	}()
	db := goqu.New("mysql8nano", mt.db.Db)
	t := time.Date(2024, 1, 2, 3, 4, 5, 123456789, time.UTC)
	_, err = db.Insert("precise").Prepared(true).Rows(goqu.Record{"t": t}).Executor().Exec()
	mt.Require().NoError(err)

	count, err := db.From("precise").Where(goqu.C("t").Eq(t)).Count()
	mt.NoError(err)
	mt.Equal(int64(1), count)
}

func (mt *mysqlTest) TestForShareRuns() {
	var entries []entry
	mt.NoError(mt.db.From("entry").ForShare(goqu.Wait).ScanStructs(&entries))
	mt.Len(entries, 10)
}

func (mt *mysqlTest) TestUpdate() {
	ds := mt.db.From("entry")
	var e entry
	found, err := ds.Where(goqu.C("int").Eq(9)).Select("id").ScanStruct(&e)
	mt.NoError(err)
	mt.True(found)
	e.Int = 11
	_, err = ds.Where(goqu.C("id").Eq(e.ID)).Update().Set(e).Executor().Exec()
	mt.NoError(err)

	count, err := ds.Where(goqu.C("int").Eq(11)).Count()
	mt.NoError(err)
	mt.Equal(int64(1), count)
}

func (mt *mysqlTest) TestUpdateReturning() {
	ds := mt.db.From("entry")
	var id uint32
	_, err := ds.Where(goqu.C("int").Eq(11)).
		Update().
		Set(goqu.Record{"int": 9}).
		Returning("id").
		Executor().ScanVal(&id)
	mt.Error(err)
	mt.EqualError(err, "goqu: dialect does not support RETURNING clause [dialect=mysql]")
}

func (mt *mysqlTest) TestDelete() {
	ds := mt.db.From("entry")
	var e entry
	found, err := ds.Where(goqu.C("int").Eq(9)).Select("id").ScanStruct(&e)
	mt.NoError(err)
	mt.True(found)
	_, err = ds.Where(goqu.C("id").Eq(e.ID)).Delete().Executor().Exec()
	mt.NoError(err)

	count, err := ds.Count()
	mt.NoError(err)
	mt.Equal(int64(9), count)

	var id uint32
	found, err = ds.Where(goqu.C("id").Eq(e.ID)).ScanVal(&id)
	mt.NoError(err)
	mt.False(found)

	e = entry{}
	found, err = ds.Where(goqu.C("int").Eq(8)).Select("id").ScanStruct(&e)
	mt.NoError(err)
	mt.True(found)
	mt.NotEqual(0, e.ID)

	id = 0
	_, err = ds.Where(goqu.C("id").Eq(e.ID)).Delete().Returning("id").Executor().ScanVal(&id)
	mt.EqualError(err, "goqu: dialect does not support RETURNING clause [dialect=mysql]")
}

func (mt *mysqlTest) TestInsertIgnore() {
	ds := mt.db.From("entry")
	now := time.Now()

	// insert one
	entries := []entry{
		{Int: 8, Float: 6.100000, String: "6.100000", Time: now, Bytes: []byte("6.100000")},
		{Int: 9, Float: 7.200000, String: "7.200000", Time: now, Bytes: []byte("7.200000")},
		{Int: 10, Float: 7.200000, String: "7.200000", Time: now, Bytes: []byte("7.200000")},
	}
	_, err := ds.Insert().Rows(entries).OnConflict(goqu.DoNothing()).Executor().Exec()
	mt.NoError(err)

	count, err := ds.Count()
	mt.NoError(err)
	mt.Equal(count, int64(11))
}

func (mt *mysqlTest) TestInsert_OnConflict() {
	ds := mt.db.From("entry")
	now := time.Now()

	// insert
	e := entry{Int: 10, Float: 1.100000, String: "1.100000", Time: now, Bool: false, Bytes: []byte("1.100000")}
	_, err := ds.Insert().Rows(e).OnConflict(goqu.DoNothing()).Executor().Exec()
	mt.NoError(err)

	// duplicate
	e = entry{Int: 10, Float: 2.100000, String: "2.100000", Time: now.Add(time.Hour * 100), Bool: false, Bytes: []byte("2.100000")}
	_, err = ds.Insert().Rows(e).OnConflict(goqu.DoNothing()).Executor().Exec()
	mt.NoError(err)

	// update
	var entryActual entry
	e2 := entry{Int: 10, String: "2.000000"}
	_, err = ds.Insert().
		Rows(e2).
		OnConflict(goqu.DoUpdate("int", goqu.Record{"string": "upsert"})).
		Executor().Exec()
	mt.NoError(err)
	_, err = ds.Where(goqu.C("int").Eq(10)).ScanStruct(&entryActual)
	mt.NoError(err)
	mt.Equal("upsert", entryActual.String)

	// update where should error
	entries := []entry{
		{Int: 8, Float: 6.100000, String: "6.100000", Time: now, Bytes: []byte("6.100000")},
		{Int: 9, Float: 7.200000, String: "7.200000", Time: now, Bytes: []byte("7.200000")},
	}
	_, err = ds.Insert().
		Rows(entries).
		OnConflict(goqu.DoUpdate("int", goqu.Record{"string": "upsert"}).Where(goqu.C("int").Eq(9))).
		Executor().Exec()
	mt.EqualError(err, "goqu: dialect does not support upsert with where clause [dialect=mysql]")
}

func (mt *mysqlTest) TestInsert_OnConflictExcluded() {
	version := mt.serverVersion()
	for _, dialect := range []string{"mysql", "mysql8"} {
		mt.Run(dialect, func() {
			if dialect == "mysql8" && !versionAtLeast(version, [3]int{8, 0, 19}) {
				mt.T().Skipf("row alias needs MySQL 8.0.19+, server is %v", version)
			}
			db := goqu.New(dialect, mt.db.Db)
			ds := db.From("entry")
			incoming := entry{Int: 1, Float: 10, String: "incoming", Time: time.Now(), Bytes: []byte("incoming")}
			_, err := ds.Insert().Rows(incoming).
				OnConflict(goqu.DoUpdate("", goqu.Record{
					"string": goqu.Excluded("string"),
					"float":  goqu.T("entry").Col("float").Add(goqu.Excluded("float")),
				})).
				Executor().Exec()
			mt.Require().NoError(err)

			var actual entry
			_, err = ds.Where(goqu.C("int").Eq(1)).ScanStruct(&actual)
			mt.Require().NoError(err)
			mt.Equal("incoming", actual.String)
			mt.InDelta(10.1, actual.Float, 0.001)

			_, err = ds.Insert().
				Cols("int", "float", "string", "time", "bool", "bytes").
				FromQuery(db.From("entry").
					Select(goqu.L("`int` + 1"), "float", goqu.V("from query"), "time", "bool", "bytes").
					Where(goqu.C("int").Eq(1))).
				OnConflict(goqu.DoUpdateExcluded("", "string")).
				Executor().Exec()
			mt.Require().NoError(err)

			_, err = ds.Where(goqu.C("int").Eq(2)).ScanStruct(&actual)
			mt.Require().NoError(err)
			mt.Equal("from query", actual.String)
			mt.InDelta(0.2, actual.Float, 0.001)
		})
	}
}

func (mt *mysqlTest) TestWindowFunction() {
	if version := mt.serverVersion(); version[0] < 8 {
		mt.T().Skipf("window functions need MySQL 8, server is %v", version)
	}

	ds := mt.db.From("entry").
		Select("int", goqu.ROW_NUMBER().OverName(goqu.I("w")).As("id")).
		Window(goqu.W("w").OrderBy(goqu.I("int").Desc()))

	var entries []entry
	mt.NoError(ds.WithDialect("mysql8").ScanStructs(&entries))

	mt.Equal([]entry{
		{Int: 9, ID: 1},
		{Int: 8, ID: 2},
		{Int: 7, ID: 3},
		{Int: 6, ID: 4},
		{Int: 5, ID: 5},
		{Int: 4, ID: 6},
		{Int: 3, ID: 7},
		{Int: 2, ID: 8},
		{Int: 1, ID: 9},
		{Int: 0, ID: 10},
	}, entries)

	mt.Error(ds.WithDialect("mysql").ScanStructs(&entries), "goqu: adapter does not support window function clause")
}

func (mt *mysqlTest) TestStraightJoin() {
	ds := mt.db.From("entry").
		StraightJoin(
			goqu.T("entry").As("e2"),
			goqu.On(goqu.I("entry.id").Eq(goqu.I("e2.id"))),
		)
	// joining a table to itself on its primary key must not change the row count
	expected, err := mt.db.From("entry").Count()
	mt.NoError(err)
	mt.Positive(expected)
	count, err := ds.Count()
	mt.NoError(err)
	mt.Equal(expected, count)

	var e entry
	found, err := ds.Select("entry.*").Where(goqu.I("e2.id").Eq(1)).ScanStruct(&e)
	mt.NoError(err)
	mt.True(found)
	mt.Equal(uint32(1), e.ID)

	// MySQL 5.7 rejects USING for STRAIGHT_JOIN, so goqu refuses to generate it
	_, err = mt.db.From("entry").StraightJoin(goqu.T("entry").As("e2"), goqu.Using("id")).Count()
	mt.EqualError(err, "goqu: USING is not supported for StraightJoinType, use an ON condition")
}

func (mt *mysqlTest) TestInsertFromSelect() {
	ds := mt.db.From("entry")

	subquery := goqu.Select(
		goqu.V(11),
		goqu.V(11),
		goqu.C("float"),
		goqu.C("string"),
		goqu.C("time"),
		goqu.C("bool"),
		goqu.C("bytes"),
	).From(goqu.T("entry")).Where(goqu.C("int").Eq(9))

	query := ds.Insert().Cols().FromQuery(subquery)
	_, _, err := query.ToSQL()

	mt.NoError(err)
	_, err = query.Executor().Exec()
	mt.NoError(err)
}

func TestMysqlSuite(t *testing.T) {
	suite.Run(t, new(mysqlTest))
}
