# Working with time.Time

By default when interpolating `time.Time` (and `*time.Time`) `goqu` will convert it `UTC` before interpolating.

## Why?

For most use cases `UTC` should be preferred, if a timezone is specified it is usually ignored silently by `postgres` and `mysql` unless you configure your DB to run in a different timezone, leading to unexpected behavior.

## How to use a different default timezone?
`goqu` provides a **_global_** configuration settings to set the [location](https://golang.org/pkg/time/#Location) to convert all timestamps to. 

To change the default timezone to covert time instances to you can use [`goqu.SetTimeLocation`](https://godoc.org/github.com/doug-martin/goqu#SetTimeLocation) to change the default timezone.

In the following example the default value `UTC` is used.

```go
created, err := time.Parse(time.RFC3339, "2019-10-01T15:01:00Z")
if err != nil {
	panic(err)
}

ds := goqu.Insert("test").Rows(goqu.Record{
	"address": "111 Address",
	"name":    "Bob Yukon",
	"created": created,
})
```

Output:
```
INSERT INTO "test" ("address", "created", "name") VALUES ('111 Address', '2019-10-01T15:01:00Z', 'Bob Yukon')
```

In the following example `UTC` is overridden to `Asia/Shanghai`

```go
loc, err := time.LoadLocation("Asia/Shanghai")
if err != nil {
	panic(err)
}

goqu.SetTimeLocation(loc)

created, err := time.Parse(time.RFC3339, "2019-10-01T15:01:00Z")
if err != nil {
	panic(err)
}

ds := goqu.Insert("test").Rows(goqu.Record{
	"address": "111 Address",
	"name":    "Bob Yukon",
	"created": created,
})
```

Output:
```
INSERT INTO "test" ("address", "created", "name") VALUES ('111 Address', '2019-10-01T23:01:00+08:00', 'Bob Yukon')
```

## Fractional seconds

When a `time.Time` is interpolated (non-prepared mode, `goqu`'s default) it is formatted with the dialect's `TimeFormat`, so the fraction that reaches the database depends on the dialect:

| dialect | interpolated fraction |
|---|---|
| `mysql`, `mysql8` | microseconds; MySQL rounds them to the column's precision, as it does for prepared values |
| `mysql8nano` | nanoseconds; MySQL rounds them to the column's precision, as it does for prepared values |
| `sqlserver` | 100 nanoseconds |
| `sqlite3` | nanoseconds as `2006-01-02 15:04:05.999999999-07:00`, the text the go-sqlite3 driver binds in prepared mode, so both modes compare equal |
| `postgres`, `duckdb`, default | nanoseconds (`time.RFC3339Nano`) |

