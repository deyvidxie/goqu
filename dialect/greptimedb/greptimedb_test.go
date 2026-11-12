package greptimedb_test

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exec"
	"github.com/doug-martin/goqu/v9/exp"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
)

const (
	dropEntry    = "DROP TABLE IF EXISTS entry"
	dropCopy     = "DROP TABLE IF EXISTS entry_copy"
	entryColumns = "(ts TIMESTAMP(9) TIME INDEX, host STRING PRIMARY KEY, cpu DOUBLE, ok BOOLEAN)"
)

var baseTime = time.Date(2024, 1, 2, 3, 4, 5, 123456789, time.UTC)

type (
	greptimedbTest struct {
		suite.Suite
		dialect string
		driver  string
		uriEnv  string
		uri     string
		db      *goqu.Database
	}
	entry struct {
		TS   string  `db:"ts"`
		Host string  `db:"host"`
		CPU  float64 `db:"cpu"`
		OK   bool    `db:"ok"`
	}
)

func (gt *greptimedbTest) SetupSuite() {
	uri := os.Getenv(gt.uriEnv)
	if uri == "" {
		uri = gt.uri
	}
	db, err := sql.Open(gt.driver, uri)
	gt.Require().NoError(err)
	gt.db = goqu.New(gt.dialect, db)
}

func (gt *greptimedbTest) SetupTest() {
	for _, stmt := range []string{
		dropEntry, dropCopy,
		"CREATE TABLE entry " + entryColumns,
		"CREATE TABLE entry_copy " + entryColumns,
	} {
		_, err := gt.db.Exec(stmt)
		gt.Require().NoError(err, stmt)
	}
	records := make([]interface{}, 5)
	for i := range records {
		records[i] = goqu.Record{
			"ts":   baseTime.Add(time.Duration(i) * time.Minute),
			"host": string(rune('a' + i)),
			"cpu":  float64(i) * 1.5,
			"ok":   i%2 == 0,
		}
	}
	_, err := gt.db.Insert("entry").Rows(records...).Executor().Exec()
	gt.Require().NoError(err)
}

func (gt *greptimedbTest) entries() *goqu.SelectDataset {
	return gt.db.From("entry").Select(goqu.Cast(goqu.C("ts"), "STRING").As("ts"), "host", "cpu", "ok")
}

func (gt *greptimedbTest) hosts(ds *goqu.SelectDataset) []string {
	var hosts []string
	gt.Require().NoError(ds.Select("host").ScanVals(&hosts))
	return hosts
}

func (gt *greptimedbTest) bothModes(run func(ds *goqu.SelectDataset, prepared bool)) {
	for _, prepared := range []bool{false, true} {
		run(gt.db.From("entry").Prepared(prepared), prepared)
	}
}

func (gt *greptimedbTest) TestSelectFilters() {
	gt.bothModes(func(ds *goqu.SelectDataset, prepared bool) {
		gt.Equal([]string{"e", "c"}, gt.hosts(ds.Where(goqu.C("ok").IsTrue(), goqu.C("cpu").Gt(1)).Order(goqu.C("host").Desc())), prepared)
		gt.Equal([]string{"b", "c"}, gt.hosts(ds.Order(goqu.C("host").Asc()).Limit(2).Offset(1)), prepared)
		gt.Equal([]string{"d", "e"}, gt.hosts(ds.Order(goqu.C("host").Asc()).Offset(3)), prepared)
		gt.Equal([]string{"a", "b"}, gt.hosts(ds.Where(goqu.C("host").In("a", "b", "z")).Order(goqu.C("host").Asc())), prepared)
		gt.Equal([]string{"b"}, gt.hosts(ds.Where(goqu.C("host").RegexpLike("^b"), goqu.C("host").ILike("B%"))), prepared)
		gt.Equal([]string{"a", "b"}, gt.hosts(ds.Where(goqu.C("host").RegexpNotILike("^[C-E]")).Order(goqu.C("host").Asc())), prepared)
		gt.Equal([]string{"b"}, gt.hosts(ds.Where(goqu.C("cpu").Between(exp.NewRangeVal(1, 2)))), prepared)
	})
}

func (gt *greptimedbTest) TestTimeRoundTrip() {
	gt.bothModes(func(ds *goqu.SelectDataset, prepared bool) {
		gt.Equal([]string{"a"}, gt.hosts(ds.Where(goqu.C("ts").Eq(baseTime))), prepared)
		gt.Equal([]string{"b"}, gt.hosts(ds.Where(goqu.C("ts").Eq(baseTime.Add(time.Minute)))), prepared)
		gt.Empty(gt.hosts(ds.Where(goqu.C("ts").Eq(baseTime.Add(time.Nanosecond)))), prepared)
	})
	var e entry
	found, err := gt.entries().Where(goqu.C("host").Eq("a")).ScanStruct(&e)
	gt.Require().NoError(err)
	gt.True(found)
	gt.Equal(entry{TS: "2024-01-02T03:04:05.123456789", Host: "a", CPU: 0, OK: true}, e)
}

func (gt *greptimedbTest) TestStringRoundTrip() {
	value := `it's a \ "quoted" \n value`
	for _, prepared := range []bool{false, true} {
		_, err := gt.db.Insert("entry").Prepared(prepared).
			Rows(goqu.Record{"ts": baseTime, "host": value, "cpu": 1, "ok": true}).Executor().Exec()
		gt.Require().NoError(err)
		gt.Equal([]string{value}, gt.hosts(gt.db.From("entry").Prepared(prepared).Where(goqu.C("host").Eq(value))), prepared)
		_, err = gt.db.Delete("entry").Where(goqu.C("host").Eq(value)).Executor().Exec()
		gt.Require().NoError(err)
	}
}

func (gt *greptimedbTest) TestTimeFunctions() {
	gt.bothModes(func(ds *goqu.SelectDataset, prepared bool) {
		gt.Len(gt.hosts(ds.Where(goqu.C("ts").Gt(goqu.Now().Sub(goqu.Interval(100000, exp.DayInterval))))), 5, prepared)
		var buckets []string
		gt.Require().NoError(ds.Select(goqu.Cast(goqu.Func("date_bin", goqu.Interval(2, exp.MinuteInterval), goqu.C("ts")), "STRING").As("b")).
			GroupBy("b").Order(goqu.C("b").Asc()).ScanVals(&buckets))
		gt.Equal([]string{"2024-01-02T03:04:00", "2024-01-02T03:06:00", "2024-01-02T03:08:00"}, buckets, prepared)
	})
}

func (gt *greptimedbTest) TestRangeQuery() {
	type bucket struct {
		TS  string  `db:"ts"`
		CPU float64 `db:"cpu"`
	}
	alignOrigin := baseTime.Truncate(time.Millisecond)
	gt.bothModes(func(ds *goqu.SelectDataset, prepared bool) {
		var buckets []bucket
		gt.Require().NoError(ds.
			Select(goqu.Cast(goqu.C("ts"), "STRING").As("ts"),
				goqu.RangeAggregate(goqu.AVG("cpu"), time.Minute).Fill(goqu.FillLinear()).As("cpu")).
			Where(goqu.C("host").In("a", "c")).
			Align(goqu.Align(time.Minute).To(alignOrigin).ByNone()).
			Order(goqu.C("ts").Asc()).ScanStructs(&buckets))
		gt.Equal([]bucket{
			{TS: "2024-01-02T03:04:05.123", CPU: 0},
			{TS: "2024-01-02T03:05:05.123", CPU: 1.5},
			{TS: "2024-01-02T03:06:05.123", CPU: 3},
		}, buckets, prepared)

		var constant []bucket
		gt.Require().NoError(ds.
			Select(goqu.Cast(goqu.C("ts"), "STRING").As("ts"), goqu.RangeAggregate(goqu.AVG("cpu"), time.Minute).As("cpu")).
			Where(goqu.C("host").In("a", "c")).
			Align(goqu.Align(time.Minute).To(alignOrigin).ByNone().Fill(goqu.FillValue(7))).
			Order(goqu.C("ts").Asc()).ScanStructs(&constant))
		gt.Equal([]bucket{
			{TS: "2024-01-02T03:04:05.123", CPU: 0},
			{TS: "2024-01-02T03:05:05.123", CPU: 7},
			{TS: "2024-01-02T03:06:05.123", CPU: 3},
		}, constant, prepared)

		var counts []int64
		gt.Require().NoError(ds.
			Select(goqu.RangeAggregate(goqu.COUNT(goqu.Star()), 5*time.Minute).As("n")).
			Align(goqu.Align(5 * time.Minute).To(alignOrigin).By("ok")).
			Order(goqu.C("n").Asc()).ScanVals(&counts))
		gt.Equal([]int64{2, 3}, counts, prepared)
	})
}

func (gt *greptimedbTest) TestWindowsAndCTEs() {
	ds := gt.db.From("entry")
	latest := goqu.ROW_NUMBER().Over(goqu.W().PartitionBy("ok").OrderBy(goqu.C("ts").Desc()))
	gt.Equal([]string{"d", "e"}, gt.hosts(ds.Qualify(latest.Eq(1)).Order(goqu.C("host").Asc())))
	gt.Equal([]string{"d", "e"}, gt.hosts(ds.DistinctOn("ok").Order(goqu.C("ok").Asc(), goqu.C("ts").Desc())))

	var ranks []int64
	gt.Require().NoError(ds.Select(goqu.ROW_NUMBER().OverName(goqu.I("w"))).Window(goqu.W("w").OrderBy(goqu.C("ts").Asc())).
		Order(goqu.C("ts").Asc()).ScanVals(&ranks))
	gt.Equal([]int64{1, 2, 3, 4, 5}, ranks)

	gt.Equal([]string{"a", "c"}, gt.hosts(gt.db.From("t").
		With("t", ds.Where(goqu.C("cpu").Lt(4))).
		Where(goqu.C("host").In(gt.db.From("entry").Select("host").Where(goqu.C("ok").IsTrue()))).
		Order(goqu.C("host").Asc())))

	var series []int64
	gt.Require().NoError(gt.db.From("r").WithRecursive("r",
		gt.db.Select(goqu.L("1").As("n")).UnionAll(gt.db.From("r").Select(goqu.L("n + 1")).Where(goqu.C("n").Lt(3))),
	).Select("n").Order(goqu.C("n").Asc()).ScanVals(&series))
	gt.Equal([]int64{1, 2, 3}, series)

	gt.Equal([]string{"a", "c"}, gt.hosts(ds.Select("host").Where(goqu.C("ok").IsTrue()).
		Intersect(ds.Select("host").Where(goqu.C("cpu").Lt(4))).Order(goqu.C("host").Asc())))
}

func (gt *greptimedbTest) TestInsertSelectDeleteTruncate() {
	_, err := gt.db.Insert("entry_copy").Cols("ts", "host", "cpu", "ok").
		FromQuery(gt.db.From("entry").Select("ts", "host", "cpu", "ok").Where(goqu.C("ok").IsTrue())).Executor().Exec()
	gt.Require().NoError(err)
	gt.Equal([]string{"a", "c", "e"}, gt.hosts(gt.db.From("entry_copy").Order(goqu.C("host").Asc())))

	for _, prepared := range []bool{false, true} {
		_, err = gt.db.Delete("entry_copy").Prepared(prepared).Where(goqu.C("host").Eq("a")).Executor().Exec()
		gt.Require().NoError(err)
	}
	gt.Equal([]string{"c", "e"}, gt.hosts(gt.db.From("entry_copy").Order(goqu.C("host").Asc())))

	_, err = gt.db.Truncate("entry_copy").Executor().Exec()
	gt.Require().NoError(err)
	gt.Empty(gt.hosts(gt.db.From("entry_copy")))
}

func (gt *greptimedbTest) TestRejectedBeforeReachingServer() {
	row := goqu.Record{"ts": baseTime, "host": "a", "cpu": 99}
	for _, executor := range []exec.QueryExecutor{
		gt.db.Update("entry").Set(goqu.Record{"cpu": 99}).Executor(),
		gt.db.Insert("entry").Rows(row).OnConflict(goqu.DoNothing()).Executor(),
		gt.db.Insert("entry").Rows(row).Returning("host").Executor(),
		gt.db.From("entry").ForUpdate(goqu.Wait).Executor(),
	} {
		_, err := executor.Exec()
		gt.True(goqu.IsUnsupported(err))
	}
	var cpu []float64
	gt.Require().NoError(gt.db.From("entry").Select("cpu").Where(goqu.C("host").Eq("a")).ScanVals(&cpu))
	gt.Equal([]float64{0}, cpu)
}

func TestGreptimeDBMySQLSuite(t *testing.T) {
	suite.Run(t, &greptimedbTest{
		dialect: "greptimedb", driver: "mysql",
		uriEnv: "GREPTIMEDB_MYSQL_URI", uri: "root@tcp(127.0.0.1:4002)/public",
	})
}

func TestGreptimeDBPGSuite(t *testing.T) {
	suite.Run(t, &greptimedbTest{
		dialect: "greptimedb-pg", driver: "postgres",
		uriEnv: "GREPTIMEDB_PG_URI", uri: "postgres://root@127.0.0.1:4003/public?sslmode=disable",
	})
}
