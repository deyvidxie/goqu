# Unreleased

* [BREAKING] Empty input no longer resets or drops a clause; it returns an `InvalidExpressionError`, and each clause has an explicit `Clear*`: `Returning()` with no columns on `InsertDataset`, `UpdateDataset` and `DeleteDataset` (new `ClearReturning`), `SelectDataset.Window()` with no windows (`ClearWindow`), `SelectDataset.From()` and `UpdateDataset.From()` with no tables (new `ClearFrom` on both; build a select without `FROM` with `goqu.Select(...)` instead of `goqu.From().Select(...)`), and `Truncate()`/`TruncateDataset.Table()` with no tables. The SQL generators reject an empty `RETURNING`, `FROM`, `UPDATE … FROM`, `TRUNCATE` or `WINDOW` list set directly with `exp`, and `HasReturning()` reports whether a returning list was set [deyvidxie#156](https://github.com/deyvidxie/goqu/issues/156), [deyvidxie#162](https://github.com/deyvidxie/goqu/issues/162), [deyvidxie#164](https://github.com/deyvidxie/goqu/issues/164), [deyvidxie#170](https://github.com/deyvidxie/goqu/issues/170)
* [BREAKING] `ForUpdate`, `ForNoKeyUpdate`, `ForShare` and `ForKeyShare` take only a wait option and lock every table; the new `ForUpdateOf`, `ForNoKeyUpdateOf`, `ForShareOf` and `ForKeyShareOf` lock the given tables and return an `InvalidExpressionError` when given none, instead of an empty `of` spread silently locking every table [deyvidxie#156](https://github.com/deyvidxie/goqu/issues/156)
* [BREAKING] `SelectDataset.ClearSelect()` resets only the column list and keeps `Distinct`/`DistinctOn`; the new `ClearDistinct()` drops them [deyvidxie#166](https://github.com/deyvidxie/goqu/issues/166)
* [FEATURE] Add `SelectDataset.ClearLock`, `ClearJoins` and `ClearWith`, `ClearWith` on `InsertDataset`, `UpdateDataset` and `DeleteDataset`, `SelectDataset.ClearFrom` and `UpdateDataset.ClearFrom`, `InsertDataset.ClearAlias` and the `ClearReturning` methods; `exp.SelectClauses` gains `ClearJoins`, and the select, insert, update and delete clauses gain `ClearCommonTables` [deyvidxie#168](https://github.com/deyvidxie/goqu/issues/168)
* [BREAKING] `DoNothing()` no longer renders insert-ignore syntax, which also turned NOT NULL, CHECK, truncation and foreign key errors into skipped or adjusted rows: `sqlite3` renders `ON CONFLICT DO NOTHING` (and returns an `UnsupportedError` with `DEFAULT VALUES` instead of switching to `INSERT OR IGNORE`), and `mysql`/`mysql8` render `ON DUPLICATE KEY UPDATE` with the first insert column set to itself, which skips duplicate keys only, though on a conflict MySQL still runs the table's BEFORE/AFTER UPDATE triggers (an `UnsupportedError` when the insert has no column list). The `SupportsInsertIgnoreSyntax` and `InsertIgnoreClause` dialect options are removed and `ConflictDoNothingAsNoOpUpdate` is added. An unknown `exp.ConflictExpression` returns an `InvalidExpressionError` instead of rendering `DO NOTHING` [deyvidxie#163](https://github.com/deyvidxie/goqu/issues/163)
* [BREAKING] A `DoNothing` conflict target set to an empty column list returns an `InvalidExpressionError` instead of rendering an untargeted `ON CONFLICT DO NOTHING` [deyvidxie#154](https://github.com/deyvidxie/goqu/issues/154)
* [BREAKING] `InsertDataset.FromQuery(nil)` and `OnConflict(nil)` return an `InvalidExpressionError` instead of clearing the clause; use the new `ClearFromQuery()` or `ClearOnConflict()`. The `FromQuery` nil message now names `ClearFromQuery` [deyvidxie#165](https://github.com/deyvidxie/goqu/issues/165)
* [BREAKING] A Go list (other than a `driver.Valuer`) where SQL needs a single value returns an `InvalidExpressionError` ("X takes a single value, got a list") instead of rendering a row `(1, 2)`: a comparison whose left side is any non-literal expression (a `Cast`, `Func`, `Case`, arithmetic, alias; a column already failed), `Between`/`NotBetween` bounds, bitwise and arithmetic operands, simple and searched `CASE` `WHEN`, `THEN` and `ELSE` values, and function arguments (`COALESCE("a", []int{1, 2})`). A one-element list still renders `(x)`, and a literal (`L("(a, b)")`) left side may still be compared with a row. The column comparison error now reads "operator X takes a single value" instead of "compares a column with a single value" [deyvidxie#148](https://github.com/deyvidxie/goqu/issues/148), [deyvidxie#150](https://github.com/deyvidxie/goqu/issues/150)
* [BREAKING] `Any`/`All` with a Go list return an `InvalidExpressionError` pointing to `In`/`NotIn`, a `driver.Valuer` array or a subquery, instead of rendering `ANY ((1, 2))`, which no supported database accepts; `exp.AnyFunctionName`/`AllFunctionName` name the two functions [deyvidxie#147](https://github.com/deyvidxie/goqu/issues/147)
* [BREAKING] `exp.NewBooleanExpression` with `InOp`/`NotInOp` and a right side that is not a list, subquery or literal (a scalar, `[]byte`, `json.RawMessage` or `driver.Valuer`) returns an `InvalidExpressionError` instead of rendering `IN 1` or `IN ''`; `In`/`NotIn` always wrap a single value in a list [deyvidxie#149](https://github.com/deyvidxie/goqu/issues/149)
* [FIX] `SetColumnRenameFunction` clears the cached struct column maps like `SetIgnoreUntaggedFields`, so a struct already used in `Select`, `Rows` or a scan picks up the new names. Both setters, and the row-struct check used by scans, now hold the column map lock, so calling a setter while queries are built or scanned is no longer a data race [deyvidxie#145](https://github.com/deyvidxie/goqu/issues/145)
* [FIX] A select with an `Offset` or a lock clause (`ForUpdate` and the like), and no `Order` or `Limit`, used in `Union`, `UnionAll`, `Intersect` or `IntersectAll` is wrapped in a subselect like one with an `Order` or `Limit`, instead of having the `OFFSET` or lock render after the compound and apply to the whole result [deyvidxie#134](https://github.com/deyvidxie/goqu/issues/134)
* [FEATURE] `sqlserver` supports window functions and common table expressions: `Over` renders, `With` and `WithRecursive` render `WITH` (T-SQL has no `RECURSIVE` keyword), and `sqlite3` supports window functions and the `WINDOW` clause. New dialect options `SupportsWindowClause` and `SupportsNestedWithCTE` (both `true` by default) are `false` on `sqlserver`, so a named `WINDOW` clause (SQL Server 2022 only) and a `WITH` inside a subquery, compound, CTE body or INSERT source return an `UnsupportedError`. The unsupported window function error now reads "window functions" instead of "WINDOW clause" [deyvidxie#130](https://github.com/deyvidxie/goqu/issues/130)
* [BREAKING] Dialect options now match what the databases accept, so these return an `UnsupportedError` instead of rendering SQL the database rejects or silently dropping a clause: `ORDER BY` and `LIMIT` on `UPDATE`, and `LIMIT` on `DELETE`, on `duckdb`; `ORDER BY` and `LIMIT` on `UPDATE` and `DELETE` on `sqlite3` (go-sqlite3 is built without `SQLITE_ENABLE_UPDATE_DELETE_LIMIT`); `ForUpdate` on `sqlite3` and `duckdb`, which used to render no lock at all [deyvidxie#133](https://github.com/deyvidxie/goqu/issues/133), [deyvidxie#157](https://github.com/deyvidxie/goqu/issues/157), [deyvidxie#172](https://github.com/deyvidxie/goqu/issues/172)
* [BREAKING] Unsupported builder input returns an `InvalidExpressionError` from `ToSQL` instead of panicking: an alias that is not a string or identifier (`C("a").As(1)`), a column value that is not a string, expression or struct (`Select(1)`), an unsupported table for `Insert(...).Into`, `Update(...).Table` and `Delete(...).From`, and a `FromQuery` select whose dialect differs from the insert's [deyvidxie#110](https://github.com/deyvidxie/goqu/issues/110)
* [BREAKING] An interpolated `string` or `[]byte` value that is not valid UTF-8 returns an `InvalidExpressionError` pointing to `Prepared(true)` instead of having its invalid bytes replaced with U+FFFD; prepared queries still pass it through unchanged. A literal (`L`) with arguments is written byte for byte instead of having invalid UTF-8 replaced, as a literal without arguments already was [deyvidxie#98](https://github.com/deyvidxie/goqu/issues/98)
* [FIX] A nil pointer to a pointer-receiver `driver.Valuer` in `Eq`, `Neq`, `Ex` and the other comparisons renders its `Value()`, as it already did in inserts and updates, instead of `IS NULL`; a nil pointer whose element type implements `driver.Valuer` still renders `IS NULL`. Both rules come from one shared helper [deyvidxie#97](https://github.com/deyvidxie/goqu/issues/97)
* [FIX] A nil `*SelectDataset` (or any nil subquery) passed to `From`, `Union`/`UnionAll`/`Intersect`/`IntersectAll`, `With`, `In`, an update `From`, or used as a value in `Set`/`Rows` returns an `InvalidExpressionError` ("subquery must not be nil") instead of panicking; so do a nil compound or insert source set directly on `exp` clauses [deyvidxie#115](https://github.com/deyvidxie/goqu/issues/115)
* [FEATURE] The SQL builder moves from `internal/sb` to the public `sqlbuilder` package (`sqlbuilder.SQLBuilder`, `sqlbuilder.New`), so `SQLDialect`, `exp.AppendableExpression.AppendSQL`, the datasets' `AppendSQL`, the `sqlgen` generators and `exec.QueryFactory.FromSQLBuilder` can be implemented and called outside the module. The builder pool stays internal [deyvidxie#94](https://github.com/deyvidxie/goqu/issues/94)
* [DOCS] List the `mysql8` and `mysql8nano` dialects and fix the source links in `docs/dialect.md`; close the `panic(err.Error())` calls in the `QueryExecutor` godoc examples; the `SetError` samples use a typed error and compile as written, and the insert sample checks `name` instead of an undefined `field` [deyvidxie#91](https://github.com/deyvidxie/goqu/issues/91), [deyvidxie#143](https://github.com/deyvidxie/goqu/issues/143), [deyvidxie#111](https://github.com/deyvidxie/goqu/issues/111)
* [BREAKING] A dialect name that is not registered no longer falls back to the default dialect. Datasets built through `goqu.Dialect`, `DialectWithTagName`, `New`, `NewWithTagName`, `NewTx`, `NewTxWithTagName` or any `WithDialect` with such a name return an `UnknownDialectError` (`goqu.IsUnknownDialect`) from `ToSQL`, and so do `ExpressionSQL`/`PreparedExpressionSQL` on a dialect wrapper, `Database` or `TxDatabase`, and `FromSQL` and the raw-SQL `Scan*` methods on such a `Database` or `TxDatabase`. `GetDialect` and `GetDialectWithTag` return `(SQLDialect, error)`. `InsertDataset.FromQuery` keeps the error of a select built with such a name instead of rendering it in the insert's dialect [deyvidxie#88](https://github.com/deyvidxie/goqu/issues/88), [deyvidxie#89](https://github.com/deyvidxie/goqu/issues/89), [deyvidxie#90](https://github.com/deyvidxie/goqu/issues/90)
* [BREAKING] Struct arguments to `goqu.W().PartitionBy`/`OrderBy` and `goqu.Using` are expanded when the SQL is generated, with the dataset's or dialect's struct tag instead of always `db`; a struct with no columns returns an `InvalidExpressionError` ("X has no columns"). `exp.NewWindowExpression` no longer takes a tag name, and `sqlgen.NewExpressionSQLGenerator` takes one [deyvidxie#80](https://github.com/deyvidxie/goqu/issues/80)
* [BREAKING] A window's `PartitionBy` or `OrderBy` given no columns (no arguments, an empty spread or `nil`) fails to render with "PartitionBy/OrderBy requires at least one column" instead of dropping the clause, and so does an empty partition or order list passed to `exp.NewWindowExpression`; `WindowExpression.HasPartitionBy()`/`HasOrder()` report whether the list was set [deyvidxie#155](https://github.com/deyvidxie/goqu/issues/155)
* [BREAKING] `SelectDataset.WindowAppend()` with no windows returns an `InvalidExpressionError` instead of being ignored, and `exp.SelectClauses.WindowsAppend()` with none fails to render with "WindowsAppend requires at least one window" [deyvidxie#167](https://github.com/deyvidxie/goqu/issues/167)
* [FIX] A nil `OrderedExpression` passed to `SetOrder`, `OrderAppend` or `OrderPrepend` on `exp.SelectClauses`, `UpdateClauses` and `DeleteClauses` fails to render with "Order expressions must not be nil" instead of panicking [deyvidxie#171](https://github.com/deyvidxie/goqu/issues/171)
* [FEATURE] Add `Database.FromSQL` and `TxDatabase.FromSQL`, returning an `exec.QueryExecutor` for raw SQL so it works with `ScanAllStructs`, `ScanOneStruct`, `ScanAllVals` and `ScanOneVal` [deyvidxie#74](https://github.com/deyvidxie/goqu/issues/74)
* [FEATURE] Add `ExpressionSQL` and `PreparedExpressionSQL` to `Database` and `TxDatabase`, rendering with their dialect and tag, and package-level `goqu.ExpressionSQL` and `goqu.PreparedExpressionSQL` for the default dialect [deyvidxie#93](https://github.com/deyvidxie/goqu/issues/93), [deyvidxie#95](https://github.com/deyvidxie/goqu/issues/95)
* [BREAKING] `Limit(0)` on `SelectDataset`, `UpdateDataset` and `DeleteDataset` renders `LIMIT 0` (`TOP (0)` on sqlserver) instead of clearing the limit; use `ClearLimit()` for no limit. `ScanStruct`, `ScanVal`, `ScanOneStruct`, `ScanOneVal` and `Count` keep a `Limit(0)` instead of replacing it with `Limit(1)`, so `Count()` on a `Limit(0)` dataset returns 0. A zero limit with an `Offset` returns an `UnsupportedError` on sqlserver and on any dialect that renders `OFFSET … FETCH` [deyvidxie#67](https://github.com/deyvidxie/goqu/issues/67), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] A dialect without LIMIT or ORDER BY on `Update`/`Delete`, including the default dialect, returns an `UnsupportedError` for them instead of dropping them, including an `Order`/`Limit` copied by `SelectDataset.Update()`/`Delete()` (`LimitAll()` is still dropped). The `ErrorOnUnsupportedOrderByOnDelete` dialect option is removed [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] `Where()` with no arguments or an empty spread on `SelectDataset`, `UpdateDataset` and `DeleteDataset`, `SelectDataset.Having()` and `Qualify()`, and an upsert `DoUpdate(...).Where()` (also after an earlier `Where(cond)`; an `UnsupportedError` on mysql and sqlserver), return an `InvalidExpressionError` instead of being ignored [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] On `exp` clauses, `SelectClauses.WhereAppend()`/`HavingAppend()`/`QualifyAppend()`/`OrderAppend()`/`OrderPrepend()` and `UpdateClauses`/`DeleteClauses.WhereAppend()`/`OrderAppend()`/`OrderPrepend()` with no arguments fail to render with "X requires at least one expression" (an `UnsupportedError` for `UpdateClauses`/`DeleteClauses` ORDER BY on dialects without it), and `SelectClauses.SelectAppend()`/`GroupByAppend()` and `InsertClauses.ColsAppend()` with an empty or nil list, and `InsertClauses.ValsAppend()` with no rows, fail with "X requires at least one column"/"row", instead of being ignored. A nil list used to panic in `SelectAppend` and `ColsAppend`, and in `GroupByAppend` when a GROUP BY was set. Appending to a nil select list, or to a select, group by, order, cols or vals list set empty with `SetSelect`/`SetGroupBy`/`SetOrder`/`SetCols`/`SetVals`, keeps the "X requires at least one …" error that the SQL generator returns for the empty list [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] `Select`, `SelectDistinct`, `SelectAppend`, `GroupBy` and `GroupByAppend` given no columns (no arguments, an empty spread, `nil`, or a struct with no columns), including `goqu.Select()`, `Database.Select()`, `TxDatabase.Select()` and a dialect wrapper's `Select()`, and `Order`, `OrderAppend` and `OrderPrepend` on `SelectDataset`, `UpdateDataset` and `DeleteDataset` given no expressions or a `nil` one, return an `InvalidExpressionError` instead of resetting to `SELECT *`, dropping `GROUP BY`/`ORDER BY`, being ignored (`SelectAppend`, `GroupByAppend`, `OrderAppend`, `OrderPrepend`) or panicking; use `ClearSelect()`, `ClearGroupBy()` or `ClearOrder()`. `InsertDataset.ColsAppend()` with no columns returns one too. The SQL generators return the same "X requires at least one column"/"expression"/"row" `InvalidExpressionError` for an empty select, group by, order, cols, rows or vals list built directly with `exp`. `SelectDataset.Update()`, `Delete()`, `Insert()` and `Truncate()` keep the select's error [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] An insert given an empty source returns an `InvalidExpressionError` instead of inserting a row of defaults: `Rows()`/`Cols()` with no arguments, `Rows` with an empty slice or array or a row that has no columns (`goqu.Record{}`, or a struct whose fields are all `skipinsert`, or all `omitempty`/`omitnil` and zero, which fails only for such values), `Vals()` with no rows, `Cols(...)` without `Vals` or `FromQuery` and without `Rows` (including `Cols` left set after `ClearVals()` or `ClearRows()`), and an empty `Rows` or `Vals` list, or an empty `Cols` list without `Rows`, set on `exp.InsertClauses` (`Cols` given with `Rows` is still ignored, see [deyvidxie#158](https://github.com/deyvidxie/goqu/issues/158)); `exp.NewInsertExpression` returns `goqu: insert rows have no columns` for such a row instead of an empty insert expression. Only an insert without `Rows`, `Vals`, `Cols` or `FromQuery` renders `DEFAULT VALUES` [deyvidxie#58](https://github.com/deyvidxie/goqu/issues/58), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] An insert with more than one of `Rows`, `Vals` and `FromQuery` returns an `InvalidExpressionError` instead of using one and dropping the others. The insert SQL generator enforces this, so `exp.InsertClauses` built directly get it too [deyvidxie#58](https://github.com/deyvidxie/goqu/issues/58), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [FEATURE] Add `SelectDataset.ClearGroupBy`, `ClearHaving` and `ClearQualify`, and `exp.SelectClauses.ClearGroupBy` [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] Remove `exp.InsertClauses.HasRows()` and `HasVals()`; compare `Rows()`/`Vals()` with `nil` [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [BREAKING] `SelectDataset.Distinct()` takes no arguments and renders a plain `DISTINCT`; pass `DISTINCT ON` columns to the new `SelectDataset.DistinctOn`, which returns an `InvalidExpressionError` when given no columns (no arguments, an empty spread, `nil` or an empty list) [deyvidxie#72](https://github.com/deyvidxie/goqu/issues/72), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [FIX] `Vals` without `Cols` renders `INSERT INTO t VALUES (…)` instead of being dropped for `DEFAULT VALUES`; every row must have the first row's length, and an empty row returns an `InvalidExpressionError` [deyvidxie#136](https://github.com/deyvidxie/goqu/issues/136), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [FIX] `Insert(...).ColsAppend` and `exp.InsertClauses.ColsAppend` without a previous `Cols` set the columns instead of panicking, and rendering `exp.SelectClauses` with a nil select list (`SetSelect(nil)`) returns an `InvalidExpressionError` instead of panicking [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [FEATURE] Add `ScanAllStructs[T]`, `ScanOneStruct[T]`, `ScanAllVals[T]` and `ScanOneVal[T]` to `SelectDataset` and `exec.QueryExecutor`: generic scans that return the rows or values, check `T` before the query and return a `nil` slice for no rows [deyvidxie#73](https://github.com/deyvidxie/goqu/pull/73)
* [FIX] Named pointer types (`type BoolRef *bool`) scan at any depth: a `BoolRef`, `*BoolRef`, named pointer to a named pointer or `sql.Null[BoolRef]` as a `ScanVal` target or struct field, a `[]*BoolRef` or `[]sql.Null[BoolRef]` for `ScanVals`, and a `BoolRef` passed to `ScanVal` itself. The value is scanned through unnamed pointers, so non-`NULL` values that needed `database/sql`'s exact-type conversions or a `sql.Scanner` (e.g. a `bool`, a `uuid.UUID`, a string into `*[]byte`) scan where they failed with a `database/sql` scan error [deyvidxie#139](https://github.com/deyvidxie/goqu/issues/139), [deyvidxie#152](https://github.com/deyvidxie/goqu/issues/152), [deyvidxie#151](https://github.com/deyvidxie/goqu/pull/151)
* [FIX] A struct with a `sql.RawBytes`, `*sql.RawBytes` or `sql.Null[sql.RawBytes]` field, directly or in a nested struct, fails `ScanStruct`, `ScanStructs`, `ScanAllStructs`, `ScanOneStruct` and `IterStructs` with an `UnsupportedScanTypeError` naming the field, before the query, instead of filling it with memory the next row or closing the rows overwrites [deyvidxie#142](https://github.com/deyvidxie/goqu/issues/142), [deyvidxie#151](https://github.com/deyvidxie/goqu/pull/151)
* [FIX] An `Op{}` with no operators, and `between`/`notBetween` in an `Op` with a value that is not a `goqu.Range`, return an `InvalidExpressionError` instead of dropping that operator (the whole condition for an empty `Op{}`) [deyvidxie#57](https://github.com/deyvidxie/goqu/issues/57), [deyvidxie#137](https://github.com/deyvidxie/goqu/pull/137), [deyvidxie#146](https://github.com/deyvidxie/goqu/pull/146)
* [FIX] A slice `driver.Valuer` (e.g. `pq.Int64Array`) is one value: `In`/`NotIn` render `IN ('{1}')` and `Eq`/`Neq`/`Ex` compare with `=`/`!=`, where they rendered `IN '{1}'`/`NOT IN '{1}'`; use `goqu.Any(pq.Int64Array{…})` for array membership [deyvidxie#59](https://github.com/deyvidxie/goqu/issues/59), [deyvidxie#146](https://github.com/deyvidxie/goqu/pull/146)
* [FIX] An operator other than `IN`/`NOT IN` comparing a column with an empty list or a list of several values (`C("a").Gt([]int{1, 2})`, `Op{"like": []string{}}`) returns an `InvalidExpressionError` instead of rendering `("a" > (1, 2))`; a one-element list still renders as a parenthesised value, and anything other than a column on the left, such as a row value `L("(a, b)")`, still takes a list [deyvidxie#64](https://github.com/deyvidxie/goqu/issues/64), [deyvidxie#146](https://github.com/deyvidxie/goqu/pull/146)
* [FIX] An empty list anywhere other than the right side of `IN`/`NOT IN` (a literal placeholder, an argument such as `Any([]int{})`/`All`, an element of an `IN` list) returns an `InvalidExpressionError` instead of rendering `()` [deyvidxie#60](https://github.com/deyvidxie/goqu/issues/60), [deyvidxie#63](https://github.com/deyvidxie/goqu/issues/63), [deyvidxie#65](https://github.com/deyvidxie/goqu/issues/65), [deyvidxie#146](https://github.com/deyvidxie/goqu/pull/146)
* [FIX] A struct whose columns cannot be built (a field that leads back to a struct containing it, or an embedded pointer to a non-struct) fails with an `InvalidExpressionError` naming the field wherever it is a column source or an insert or update row, instead of hanging or panicking; `ScanStruct`/`ScanStructs` and the generic scans of such a type fail with an `UnsupportedScanTypeError` naming the field. Adds `exp.InvalidExpression` [deyvidxie#82](https://github.com/deyvidxie/goqu/issues/82), [deyvidxie#144](https://github.com/deyvidxie/goqu/pull/144)
* [FIX] A nil `*T` or `*[]T`, and an array or pointer to an array of structs, derive the struct's columns as a column source instead of panicking. `SelectDataset.ScanStruct`/`ScanStructs` and `exec.Scanner.ScanStruct` with a nil `*T` or `*[]T` fail with `scan target must not be nil` instead of panicking, and `exec.Scanner.ScanStruct` with a non-pointer target fails with `cannot scan into T, a pointer to a struct is required` [deyvidxie#108](https://github.com/deyvidxie/goqu/issues/108), [deyvidxie#119](https://github.com/deyvidxie/goqu/issues/119), [deyvidxie#144](https://github.com/deyvidxie/goqu/pull/144)
* [FIX] `ScanStruct` and `ScanStructs` fail with an `UnsupportedScanTypeError` naming the column and field, instead of panicking, when a returned column sits behind a nil pointer to an unexported embedded struct (`SelectDataset.ScanStructs` with the default select, and the generic scans, reject such a type before the query) [deyvidxie#85](https://github.com/deyvidxie/goqu/issues/85), [deyvidxie#144](https://github.com/deyvidxie/goqu/pull/144)
* [FIX] `ScanVals`/`ScanVal` check the target before the query runs, with the same rules as `ScanAllVals`/`ScanOneVal`, and return an `UnsupportedScanTypeError` for a target they cannot fill. `ScanVals` into `[]*T` gives `nil` for `NULL`. `ScanVal` accepts named byte slices such as `json.RawMessage`, and `[]any` [deyvidxie#77](https://github.com/deyvidxie/goqu/issues/77), [deyvidxie#79](https://github.com/deyvidxie/goqu/issues/79), [deyvidxie#83](https://github.com/deyvidxie/goqu/issues/83), [deyvidxie#84](https://github.com/deyvidxie/goqu/issues/84), [deyvidxie#86](https://github.com/deyvidxie/goqu/issues/86), [deyvidxie#120](https://github.com/deyvidxie/goqu/issues/120), [deyvidxie#138](https://github.com/deyvidxie/goqu/pull/138)
* [BREAKING] An empty `And()`, `Or()`, `Ex{}` or `ExOr{}` returns an `InvalidExpressionError` wherever it renders (`Where`, `Having`, `On`, `Qualify`, CASE WHEN, a literal argument, an upsert `DoUpdate(...).Where`, or nested in another group; an upsert WHERE is an `UnsupportedError` on mysql and sqlserver) instead of being dropped. `exp.NewExpressionList`, `And`, `Or` and `Append` keep empty members instead of dropping them, so `And(Ex{}).IsEmpty()` is `false` and `Expressions()` counts them [deyvidxie#56](https://github.com/deyvidxie/goqu/issues/56), [deyvidxie#66](https://github.com/deyvidxie/goqu/issues/66), [deyvidxie#68](https://github.com/deyvidxie/goqu/issues/68), [deyvidxie#70](https://github.com/deyvidxie/goqu/issues/70), [deyvidxie#137](https://github.com/deyvidxie/goqu/pull/137), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [FIX] `goqu.Default()` renders `DEFAULT` only as an INSERT `VALUES` or UPDATE `SET` value (also through `defaultifempty`) and fails with an `InvalidExpressionError` anywhere else; sqlite3, which rejects `DEFAULT` as a value, returns an `UnsupportedError`. `Default()` now returns an `exp.DefaultExpression`, rendered through the new `DefaultFragment` dialect option [deyvidxie#51](https://github.com/deyvidxie/goqu/issues/51), [deyvidxie#135](https://github.com/deyvidxie/goqu/pull/135)
* [FIX] An INSERT or UPDATE value that is a slice or array (other than `[]byte`/`json.RawMessage`, a byte array, or a `driver.Valuer`, e.g. `pq.StringArray`) returns an `InvalidExpressionError` instead of rendering `(…)` [deyvidxie#62](https://github.com/deyvidxie/goqu/issues/62), [deyvidxie#135](https://github.com/deyvidxie/goqu/pull/135)
* [FIX] A `Vals` row whose length differs from its `Cols`, including an empty row, returns an `InvalidExpressionError` instead of rendering it [deyvidxie#61](https://github.com/deyvidxie/goqu/issues/61), [deyvidxie#135](https://github.com/deyvidxie/goqu/pull/135)
* [FIX] TRUNCATE returns an `UnsupportedError` (`goqu.IsUnsupported`) for options the dialect lacks instead of rendering them: `RESTART`/`CONTINUE IDENTITY` on mysql, mysql8, mysql8nano, sqlserver, duckdb and sqlite3; `CASCADE`/`RESTRICT` on mysql, mysql8, mysql8nano, sqlserver and sqlite3 (sqlite3 used to drop them); several tables on mysql, mysql8, mysql8nano, sqlserver, duckdb and sqlite3. sqlserver renders `TRUNCATE TABLE` [deyvidxie#48](https://github.com/deyvidxie/goqu/issues/48), [deyvidxie#132](https://github.com/deyvidxie/goqu/pull/132)
* [FIX] `Insert(...).As(alias)` fails with an `UnsupportedError` on dialects without the `INSERT … AS alias` row alias (those whose `ExcludedStyle` is not `ExcludedRowAliasStyle`: all but mysql8 and mysql8nano), and on mysql8/mysql8nano for an `INSERT … SELECT` without `Cols`; with `Cols(...).FromQuery(...)` the alias names a derived table (`SELECT * FROM (…) AS alias (cols)`) [deyvidxie#54](https://github.com/deyvidxie/goqu/issues/54), [deyvidxie#132](https://github.com/deyvidxie/goqu/pull/132)
* [FIX] mysql, mysql8, mysql8nano and sqlite3 drop `LimitAll` instead of rendering `LIMIT ALL`, and render `LIMIT 18446744073709551615` (mysql dialects) or `LIMIT -1` (sqlite3) before an `OFFSET` without a limit, where they rendered a bare `OFFSET` both servers reject [deyvidxie#131](https://github.com/deyvidxie/goqu/issues/131), [deyvidxie#132](https://github.com/deyvidxie/goqu/pull/132)
* [FEATURE] Add the `SupportsTruncateIdentity`, `SupportsTruncateCascade`, `SupportsMultipleTruncateTables` and `OffsetWithoutLimitFragment` dialect options [deyvidxie#48](https://github.com/deyvidxie/goqu/issues/48), [deyvidxie#131](https://github.com/deyvidxie/goqu/issues/131), [deyvidxie#132](https://github.com/deyvidxie/goqu/pull/132)
* [FIX] sqlserver and mysql (MySQL 5.x) return an `UnsupportedError` (`goqu.IsUnsupported`) for `goqu.Lateral` instead of rendering `LATERAL`, which neither server accepts; mysql8 and mysql8nano still render it [deyvidxie#45](https://github.com/deyvidxie/goqu/issues/45), [deyvidxie#129](https://github.com/deyvidxie/goqu/pull/129)
* [FIX] sqlserver returns an `UnsupportedError` (`goqu.IsUnsupported`) for an `Offset` without `Order` instead of dropping both the offset and the limit [deyvidxie#40](https://github.com/deyvidxie/goqu/issues/40), [deyvidxie#128](https://github.com/deyvidxie/goqu/pull/128)
* [FEATURE] Add the `OffsetRowsFragment` and `FetchRowsOnlyFragment` dialect options for the `ROWS`/`ROWS ONLY` keywords of `OFFSET … FETCH` [deyvidxie#47](https://github.com/deyvidxie/goqu/issues/47), [deyvidxie#128](https://github.com/deyvidxie/goqu/pull/128)
* [FIX] sqlite3 and duckdb return an `UnsupportedError` (`goqu.IsUnsupported`) for `NOWAIT`, `SKIP LOCKED` and `OF` in a locking clause instead of dropping them; a plain `ForUpdate` is still dropped [deyvidxie#39](https://github.com/deyvidxie/goqu/issues/39), [deyvidxie#127](https://github.com/deyvidxie/goqu/pull/127)
* [FIX] `FullJoin`, `FullOuterJoin` and `NaturalFullJoin` return an `UnsupportedError` (`goqu.IsUnsupported`) on mysql, mysql8 and mysql8nano instead of rendering `FULL JOIN`, which MySQL rejects or parses as an alias of the left table followed by an inner join [deyvidxie#52](https://github.com/deyvidxie/goqu/issues/52), [deyvidxie#126](https://github.com/deyvidxie/goqu/pull/126)
* [FEATURE] Add `IterStructs[T]` and `IterVals[T]` to `SelectDataset` and `exec.QueryExecutor`: `iter.Seq2[T, error]` iterators that scan one row at a time and close the rows when the loop ends or breaks [deyvidxie#75](https://github.com/deyvidxie/goqu/issues/75), [deyvidxie#125](https://github.com/deyvidxie/goqu/pull/125)
* [FEATURE] Add `goqu.Now`, `goqu.UTCTimestamp` and `goqu.Interval`, rendered per dialect through `SQLDialectOptions.TimeStyle` [deyvidxie#37](https://github.com/deyvidxie/goqu/issues/37), [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)
* [BREAKING] A `DoUpdate`/`DoUpdateExcluded` target on a dialect without conflict targets (mysql, mysql8, mysql8nano) returns an `UnsupportedError` (`goqu.IsUnsupported`) instead of being dropped; pass an empty target there [deyvidxie#41](https://github.com/deyvidxie/goqu/issues/41), [deyvidxie#103](https://github.com/deyvidxie/goqu/pull/103), [deyvidxie#153](https://github.com/deyvidxie/goqu/pull/153)
* [FIX] Every error goqu creates is typed and matched from the root package: `goqu.IsInvalidExpression` (bad builder input), `goqu.IsUnsupported` (a feature the dialect lacks), `goqu.IsUnsupportedScanType` (a scan target goqu cannot fill, also when found after the query has run), `goqu.IsColumnMismatch` (the query returned a column the scan target has no field for), `goqu.IsUnknownDialect` and `goqu.IsNoDatabase` (executing a dataset built without a `Database`); each kind is also an exported type for `errors.As` [deyvidxie#43](https://github.com/deyvidxie/goqu/issues/43), [deyvidxie#46](https://github.com/deyvidxie/goqu/issues/46), [deyvidxie#49](https://github.com/deyvidxie/goqu/issues/49), [deyvidxie#50](https://github.com/deyvidxie/goqu/issues/50), [deyvidxie#53](https://github.com/deyvidxie/goqu/issues/53), [deyvidxie#106](https://github.com/deyvidxie/goqu/issues/106), [deyvidxie#105](https://github.com/deyvidxie/goqu/pull/105), [deyvidxie#107](https://github.com/deyvidxie/goqu/pull/107), [deyvidxie#114](https://github.com/deyvidxie/goqu/pull/114), [deyvidxie#116](https://github.com/deyvidxie/goqu/issues/116), [deyvidxie#121](https://github.com/deyvidxie/goqu/pull/121)
* [FIX] A nil or nil-pointer insert row, a nil-pointer update value (or `nil` passed to `exp.NewUpdateExpressions`), or a nil-pointer `FromQuery` source, returns an `InvalidExpressionError` instead of panicking, and `Executor()` on a dataset built without a `Database` returns an executor that fails with a `NoDatabaseError` instead of panicking [deyvidxie#107](https://github.com/deyvidxie/goqu/pull/107)
* [FIX] `ScanStructs`/`ScanStruct` reject a target whose rows are not row structs (`sql.Null*`, `time.Time`, structs with no scannable fields, non-struct elements) with an `UnsupportedScanTypeError`; on a dataset this happens before auto-selecting columns, which used to select the wrong columns or panic [deyvidxie#78](https://github.com/deyvidxie/goqu/issues/78), [deyvidxie#76](https://github.com/deyvidxie/goqu/issues/76), [deyvidxie#124](https://github.com/deyvidxie/goqu/pull/124)
* [FIX] An insert without `Rows`, `Vals`, `Cols` or `FromQuery` renders `() VALUES ()` on mysql, mysql8 and mysql8nano and `DEFAULT VALUES` on sqlite3 and sqlserver (was `INSERT INTO t` with nothing after it); on sqlite3 an upsert on an insert that renders `DEFAULT VALUES` returns an `UnsupportedError` unless it is an untargeted `DoNothing`, rendered as `INSERT OR IGNORE` (new `SupportsConflictWithDefaultValues` dialect option) [deyvidxie#42](https://github.com/deyvidxie/goqu/issues/42), [deyvidxie#123](https://github.com/deyvidxie/goqu/pull/123)
* [FIX] `Insert().Rows` expands an array of rows, or a pointer to a slice or array of rows, like a slice [deyvidxie#118](https://github.com/deyvidxie/goqu/issues/118), [deyvidxie#122](https://github.com/deyvidxie/goqu/pull/122)
* [FIX] Go arrays are lists in expressions like slices: `In`/`NotIn`, `Eq`/`Neq`/`Ex` (→ `IN`/`NOT IN`) and any value rendered as `(…)` (literal and function arguments; one argument per element in prepared mode); `[0]T` renders the empty-list fragment; byte arrays (`[16]byte`) and array `driver.Valuer`s stay single values [deyvidxie#71](https://github.com/deyvidxie/goqu/issues/71), [deyvidxie#117](https://github.com/deyvidxie/goqu/pull/117)
* [FIX] Expression constructors, `In`/`NotIn`, `Eq`/`Neq` with a slice, `Window`, `Rows` and `Vals` copy the caller's slices, and datasets or `CASE` expressions built from one parent no longer overwrite each other's CTEs, compounds, windows or branches [deyvidxie#101](https://github.com/deyvidxie/goqu/issues/101), [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)
* [FIX] sqlite3 interpolates a `time.Time` in go-sqlite3's prepared-mode format [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)
* [FIX] mysql/mysql8 interpolate a `time.Time` with microseconds instead of dropping the fraction [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)
* [FIX] sqlite3 rejects case-sensitive `Like`/`NotLike` and makes regexp `ILike`/`NotILike` case-insensitive [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)
* [FIX] sqlserver rejects regexp matching instead of rendering `REGEXP BINARY` [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)
* [FIX] Unsupported-feature errors name the dialect, and the JSON one names the function used [deyvidxie#101](https://github.com/deyvidxie/goqu/issues/101), [deyvidxie#100](https://github.com/deyvidxie/goqu/pull/100)

## Behavior changes to be aware of

* Comparing a column with a list of several values (`C("p").Gt([]int{1, 2})`, `Op{"gte": []int{1, 2}}`) fails with `goqu: operator "gt" compares a column with a single value, got a list ([]int)` where it used to render `("p" > (1, 2))`, which PostgreSQL accepts for a composite-type column and DuckDB for a STRUCT column; write the left side as a literal, e.g. `L("?", I("p")).Gt([]int{1, 2})`.
* A literal placeholder bound to an empty list (`L("a IN ?", []int{})`) fails with `goqu: an empty list ([]int) is only accepted as the whole right side of In or NotIn` where it used to render `a IN ()`, which SQLite accepts; use `C("a").In([]int{})`, which renders the dialect's empty-IN fragment.
* `ScanVal(&raw)` with a `sql.RawBytes` fails with `goqu: ScanVal cannot return sql.RawBytes, which is only valid until the next row is read or the rows are closed; use []byte`, an `UnsupportedScanTypeError`, where it used to return bytes the closed rows had already released; `ScanVals` (and `Pluck`) into `[]sql.RawBytes` fails the same way. Both used to succeed when no row came back.
* A struct with a `sql.RawBytes`, `*sql.RawBytes` or `sql.Null[sql.RawBytes]` field fails the struct scans with `` goqu: cannot scan rows into T: field F holds a sql.RawBytes, which is only valid until the next row is read or the rows are closed; use []byte or tag it `db:"-"` ``, an `UnsupportedScanTypeError`, before the query (`exec.Scanner.ScanStructs`: before it reads a row; `exec.Scanner.ScanStruct`, which fills one row at a time, still accepts it), where it used to fill it with memory the next row or closing the rows overwrites, or succeed when the column was not selected or no row came back.
* `ScanVals` (and `Pluck`, and `exec.Scanner.ScanVals`) into `[]*T`, named or not, gives a `nil` element for `NULL`, without calling a `sql.Scanner`'s `Scan`. When `T` is a `sql.Scanner` that accepts `NULL`, `[]byte`, `any` or a pointer, it used to give a non-nil pointer, so `v.Valid` on a `[]*sql.NullString` element now panics; for other `T`, or a `sql.Scanner` that rejects `NULL`, it used to fail with a `database/sql` scan error.
* `ScanVal` targets and `ScanVals` (and `Pluck`) elements of these types fail with an `UnsupportedScanTypeError` before the query runs (`exec.Scanner.ScanVals`: before it reads a row): a non-byte slice other than `[]any` and a `sql.Scanner` (for `ScanVal`, a pointer to one, e.g. `ScanVal(&p)` with `p` a `*[]int64`); an array other than `uuid.UUID` and a `sql.Scanner`; a struct, other than a `sql.Scanner`, with a column named by a struct tag (`db` by default); a channel, function, `unsafe.Pointer`, `uintptr` or complex number; a `sql.Scanner` interface; or a `database/sql` struct that is not a `sql.Scanner`, e.g. a `*sql.Rows` cursor, which `ScanVal` used to return already closed with its parent rows. They used to run the query and fail with a `database/sql` scan error, or succeed when no row came back, when the value was `NULL` behind a pointer or inside `sql.Null[…]`, or when the driver returned a value of that type (e.g. duckdb's `Interval` under a `json` tag name).
* `ScanVals` with a nil target or one that is not a pointer to a slice, and `ScanVal` with a nil pointer, fail before the query runs where they used to run it first (a nil `ScanVal` pointer succeeded when no row came back). Like every target above that is rejected before the query, they no longer execute the statement of an `Insert`/`Update`/`Delete(...).Returning(...).Executor()`.
* `exec.Scanner.ScanVal` with a nil or non-pointer target fails with an `UnsupportedScanTypeError` (`scan target must not be nil, got T`, `cannot scan into T, a pointer is required`) where it used to return the `database/sql` scan error (`destination pointer is nil`, `destination not a pointer`), or panic for a nil `sql.Scanner` pointer; a non-pointer `sql.Scanner` with a value receiver used to scan.
* `ScanStruct`/`ScanStructs` on a `Database`, `TxDatabase`, `QueryExecutor` or a `SelectDataset` with an explicit `Select`, into a struct whose columns cannot be built (a field that leads back to a struct containing it, or an embedded pointer to a non-struct), fail before the query runs where they used to run it and succeed when no row came back; so does `SelectDataset.ScanStructs` with the default select into a struct with an embedded pointer to an unexported struct, and `exec.Scanner.ScanStructs` fails before it reads a row, even when none comes back. They no longer execute the statement of an `Insert`/`Update`/`Delete(...).Returning(...).Executor()`.
* An empty `And()`, `Or()`, `Ex{}` or `ExOr{}` returns an `InvalidExpressionError` where it used to be dropped (a `Where` holding only empty groups rendered no WHERE, so an UPDATE or DELETE wrote every row) or render nothing. `Join(t, goqu.On(goqu.Ex{}))` fails with the empty-AND error instead of `goqu: join condition required for conditioned join <JoinType>`; `On()` with no arguments still fails with it.
* `goqu.Ex{"a": goqu.Op{}}` (an `Op` with no operators) fails with `goqu: Op for "a" has no operators`, an `InvalidExpressionError`, where it used to render nothing (a bare `WHERE`, or invalid SQL, when it was the `Ex`'s only key). `between`/`notBetween` with a value that is not a `goqu.Range` fails with `goqu: Op "between" takes a goqu.Range value, got T`, an `InvalidExpressionError`, where only the `between` branch used to be dropped (a bare `WHERE` when it was the only condition).
* `goqu.Default()` and `exp.Default()` return `exp.DefaultExpression` instead of `exp.LiteralExpression`; use `goqu.L("DEFAULT")` for the old literal. `Default()` outside an INSERT or UPDATE value fails with `goqu: Default is only valid as an INSERT or UPDATE SET value`, and on sqlite3 (also through `defaultifempty`) with `goqu: dialect does not support DEFAULT as an INSERT or UPDATE value [dialect=sqlite3]`, where both used to render `DEFAULT`.
* A slice INSERT or UPDATE value that is not a `driver.Valuer` fails with `goqu: a list (…) is not an INSERT or UPDATE value; …` where it used to render `(…)`. A non-byte array value fails the same way; it used to fail with an encoding error when interpolated and bind as one argument when prepared.
* With `Cols`, `rows with different value length` compares every `Vals` row with the number of `Cols` (was: with the first row), so the message can report different numbers and an empty or mismatched first row now fails; a literal in `Cols` that names several columns counts as one.
* TRUNCATE with `Identity`, `Cascade`, `Restrict` or several tables fails with an `UnsupportedError` on dialects that lack the option (see the [FIX] entry) where it used to render SQL the server rejects; on sqlite3, `Cascade` and `Restrict` used to be dropped. sqlserver renders `TRUNCATE TABLE "t"` instead of `TRUNCATE "t"`.
* `Insert(...).As(alias)` fails with `goqu: dialect does not support an INSERT row alias [dialect=…]` on every dialect but mysql8 and mysql8nano; it used to render `… AS alias` on all of them. On mysql8 and mysql8nano an `INSERT … SELECT` with `As` renders `SELECT * FROM (…) AS alias (cols)` (it used to append `AS alias` after the select) and, without `Cols`, fails with `goqu: dialect does not support an INSERT row alias with INSERT ... SELECT without Cols(...).FromQuery(...) [dialect=…]`.
* On mysql, mysql8, mysql8nano and sqlite3, `LimitAll()` renders no limit and `Offset(n)` without `Limit` renders `LIMIT 18446744073709551615 OFFSET n` (mysql dialects) or `LIMIT -1 OFFSET n` (sqlite3); both used to render SQL the server rejects.
* `goqu.Lateral` on sqlserver and mysql fails with `goqu: dialect does not support lateral expressions [dialect=…]`, an `UnsupportedError`, where it used to render `LATERAL (…)` SQL the server rejects.
* sqlserver `Offset(n)` without `Order(...)` fails with `goqu: dialect does not support OFFSET without ORDER BY [dialect=sqlserver]`, an `UnsupportedError`, where it used to render the query without the offset or the limit.
* sqlite3 and duckdb fail `ForUpdate(goqu.NoWait)`, `ForUpdate(goqu.SkipLocked)` and `ForUpdate(…, tables…)` with `goqu: dialect does not support NOWAIT|SKIP LOCKED|OF in a locking clause [dialect=…]`, an `UnsupportedError`, where they used to render a plain `SELECT` (or, for `SKIP LOCKED`, invalid SQL).
* `FullJoin`, `FullOuterJoin` and `NaturalFullJoin` on mysql, mysql8 and mysql8nano fail with an `UnsupportedError` (`goqu: dialect does not support <JoinType> [dialect=…]`) where they used to generate `FULL [OUTER] JOIN` SQL.
* `Insert().Rows(...)` with rows that have no columns (e.g. `Rows(goqu.Record{})`, or structs whose fields are all `skipinsert`) fails with `goqu: insert rows have no columns`, an `InvalidExpressionError` (was a single `DEFAULT VALUES` row on postgres, duckdb and the default dialect, and `INSERT INTO t` with nothing after it on mysql, sqlite3 and sqlserver).
* The panic from `InsertDataset.FromQuery` with a `SelectDataset` of another dialect is now an `InvalidExpressionError` and its message starts with `goqu: `.
* `exp.NewRecordFromStruct` with a non-struct fails with `goqu: cannot create a record from <type>, a struct is required` (was `goqu: cannot scan into this type: <type>`, or a panic for a slice of structs); a pointer to a struct, which used to panic, is dereferenced, and `nil` or a nil pointer fails with `goqu: record source must not be nil, got <type>` (`nil` used to return a nil record and no error).
* `goqu.ErrBadFromArgument`, `ErrUnsupportedIntoType`, `ErrUnsupportedUpdateTableType`, `ErrQueryFactoryNotFoundError` and every exported `sqlgen.Err*` value and constructor are removed; match errors with the `goqu.Is*` helpers or `errors.As` on the `goqu` error types.
* A `driver.Valuer` whose `Value()` fails makes `ToSQL` return an `InvalidExpressionError` `goqu: driver.Valuer <type>: <cause>` that wraps the cause (was the cause unchanged) [deyvidxie#99](https://github.com/deyvidxie/goqu/issues/99).
* Reworded messages: `rows must all be the same type, expected …` (was `rows must be all the same type expected …`); `rows with different value length, expected …` and `rows with different keys, expected …` (were without the comma); `unsupported insert row type <type>, a map, goqu.Record or struct is required` (was `unsupported insert must be map, goqu.Record, or struct type got: <type>`); `unsupported update value type <type>, a map, goqu.Record or struct is required` (was `unsupported update interface type <type>`); `unsupported column type <type>, a string or LiteralExpression is required` (was `unexpected col type must be string or LiteralExpression received <type>`); `an empty identifier …` (was `a empty identifier …`); `window expression has no valid name` (was `expresion`); `no set values found when generating update sql` (was `UPDATE sql`); `dialect does not support multiple tables in UPDATE [dialect=<name>]` (was `<name> dialect does not support …`); `no database to execute the query, create the dataset with goqu.Database#From` (was `unable to execute query did you use goqu.Database#From to create the dataset`); `unsupported DeleteDataset#From argument <type>, a string or identifier expression is required` (was `unsupported DeleteDataset#From argument, a string or identifier expression is required`); `unsupported table type <type>, a string or expression is required` (was `unsupported table type, a string or identifier expression is required`); the scan-target errors name the target, e.g. `cannot scan into <type>, a pointer to a struct is required` (was `type must be a pointer to a struct when scanning into a struct`), `cannot scan into <type>, a pointer to a slice is required` (was `type must be a pointer to a slice when scanning into structs`/`vals`), `cannot scan into <type>, a pointer is required` (was `type must be a pointer when scanning into val`) and `cannot scan one value into <type>, use ScanVals for a slice` (was `type cannot be a pointer to a slice when scanning into val`), and a nil scan target fails with `scan target must not be nil, got <type>`; `cannot scan rows into <type>, a struct is required` (was `cannot scan into this type: <type>`); `unsupported Op key "<key>"` (was `unsupported expression type <key>`); column names in the scan errors are quoted with `%q` (was `"%s"`); `dialect does not support conflict target clause` (was `Conflict Target clause`); `… implements sql.Scanner (the rows that ScanStructs, ScanAllStructs, ScanOneStruct and IterStructs allocate leave it nil)` (was `… implements sql.Scanner (ScanStructs cannot do this)`).
* An unencodable value now fails with `goqu: unable to encode value …` (was `goqu_encode_error: Unable to encode value …`), an `InvalidExpressionError`.
* sqlite3 interpolates a `time.Time` as `2006-01-02 15:04:05.999999999-07:00` (was `time.RFC3339Nano`). Comparisons against rows stored in the old format are wrong: `Eq`/`In` never match and same-day ranges misorder. Normalize once:
  ```sql
  BEGIN;
  UPDATE t SET c = replace(c, 'T', ' ') WHERE c GLOB '????-??-??T*';
  UPDATE t SET c = substr(c, 1, length(c) - 1) || '+00:00' WHERE c GLOB '????-??-?? *Z';
  COMMIT;
  ```
* mysql and mysql8 interpolate microseconds instead of dropping the fraction, so a second-precision column stores the value rounded rather than truncated. `Eq` must reduce the value the way the row was stored, see [docs/time.md](docs/time.md#fractional-seconds); upserts keyed on such a column can insert a new row for an old value with a fraction of .5 or more.
* sqlite3 `Like`/`NotLike` with anything other than a `*regexp.Regexp`, and `Op{"like"}`/`Op{"notlike"}`, return an `UnsupportedError`; use `ILike`/`NotILike`, or `Like` with a `*regexp.Regexp`.
* sqlite3 regexp `ILike`/`NotILike` render `REGEXP '(?i)' || pattern`, so the registered `REGEXP` function must accept `(?i)`, see [docs/dialect.md](docs/dialect.md).
* sqlserver regexp matching returns an `UnsupportedError`.
* Unsupported-feature errors end with `[dialect=…]` and read `dialect does not support …`: operators (`dialect does not support <kind> operator "…"`, was `<kind> operator '…' not supported` for boolean, bitwise and arithmetic and `range operator … not supported` for range), the boolean data type (was `boolean data type is not supported by dialect "x"`), join types (was without the `[dialect=…]` suffix), `USING` on `STRAIGHT_JOIN` (`dialect does not support USING for <JoinType>, use an ON condition`, was `USING is not supported for <JoinType>, use an ON condition`) and SQL fragments (`dialect does not support <type> SQL fragment <fragment>`, was `unsupported <type> SQL fragment <fragment>`).

# v9.23.0

* [FEATURE] Add `SelectDataset.Qualify` and the `SupportsQualify` dialect option for the `QUALIFY` clause (Teradata, Snowflake, BigQuery, ...). Off by default for all built in dialects [#343](https://github.com/doug-martin/goqu/issues/343), [#401](https://github.com/doug-martin/goqu/pull/401) - [@slessard](https://github.com/slessard)
* [FEATURE] Add `SelectDataset.CustomJoin` to write a join clause goqu does not model (e.g. ClickHouse `ARRAY JOIN`) [#423](https://github.com/doug-martin/goqu/issues/423), [#430](https://github.com/doug-martin/goqu/pull/430) - [@darwin67](https://github.com/darwin67)

# v9.22.0

* [PERF] Reduce allocations when generating SQL: pooled SQL builders, typed fast paths for common slice types, bytewise placeholder scanning and pre-sized buffers. Roughly 10-25% faster `ToSQL` with up to 40% fewer allocations in the benchmarks [#444](https://github.com/doug-martin/goqu/pull/444) - [@pquerna](https://github.com/pquerna), [@funkyshu](https://github.com/funkyshu)
* [FIX] Generate SQL for `json.RawMessage` values (and pointers to byte slices) like `[]byte` instead of a list of numbers, and do not turn `Eq`/`Neq`/`In`/`NotIn` on them into a list comparison [#350](https://github.com/doug-martin/goqu/issues/350), [#351](https://github.com/doug-martin/goqu/pull/351) - [@lefinal](https://github.com/lefinal), [@funkyshu](https://github.com/funkyshu)
* [FIX] `ScanStruct` can scan into struct fields of a non-empty interface type such as `sql.Scanner` when the field already holds a non-nil pointer implementing `sql.Scanner` [#399](https://github.com/doug-martin/goqu/pull/399) - [@arkamar](https://github.com/arkamar), [@funkyshu](https://github.com/funkyshu)
* [DOCS] Update the Docker Compose instructions for running the tests and raise the database wait timeouts [#341](https://github.com/doug-martin/goqu/issues/341), [#480](https://github.com/doug-martin/goqu/pull/480)

## Behavior changes to be aware of

* A nil `json.RawMessage` is now written as `NULL` (it used to be an empty string, which is not valid JSON) and compares with `IS NULL` / `IS NOT NULL` in `Eq`/`Neq`.
* `In`/`NotIn` with a single `[]byte` or `json.RawMessage` argument now treats it as one value (`IN ('...')`) instead of generating `IN '...'`.
* `ScanStruct` into a `nil` `sql.Scanner` interface field (or a non-pointer value) now returns a descriptive error instead of the `database/sql` error `unsupported Scan, storing driver.Value type ... into type *sql.Scanner`.

# v9.21.0

* [FEATURE] Add `EXISTS` support: `goqu.Exists(subquery)` and `SelectDataset.Exists()` [#204](https://github.com/doug-martin/goqu/issues/204), [#412](https://github.com/doug-martin/goqu/pull/412), [#477](https://github.com/doug-martin/goqu/pull/477) - [@Yakov-Varnaev](https://github.com/Yakov-Varnaev), [@funkyshu](https://github.com/funkyshu)
* [FEATURE] Add a `Collate` expression (`goqu.Collate(expr, collation)` / `identifier.Collate(collation)`) usable in `WHERE`, `ORDER BY`, etc., with per-dialect collation quoting [#405](https://github.com/doug-martin/goqu/pull/405), [#478](https://github.com/doug-martin/goqu/pull/478) - [@tanner-lai](https://github.com/tanner-lai), [@funkyshu](https://github.com/funkyshu)
* [FEATURE] Add a `NULLIF` expression helper [#406](https://github.com/doug-martin/goqu/issues/406), [#407](https://github.com/doug-martin/goqu/pull/407) - [@PJHRobles](https://github.com/PJHRobles)
* [FEATURE] Allow conflict targets in `ON CONFLICT DO NOTHING` via `goqu.DoNothing().SetCols(...)`. Dialects without conflict target support (MySQL, SQL Server) return an error when a target is set. [#422](https://github.com/doug-martin/goqu/pull/422) - [@pedrohavay](https://github.com/pedrohavay), [@funkyshu](https://github.com/funkyshu)
* [FEATURE] MySQL: add `STRAIGHT_JOIN` support via `SelectDataset.StraightJoin` (requires an `ON` condition) [#442](https://github.com/doug-martin/goqu/issues/442), [#453](https://github.com/doug-martin/goqu/pull/453) - [@vesquen](https://github.com/vesquen)
* [FEATURE] Prepared statements: pass any driver-bindable value (structs, maps, byte arrays such as UUIDs) through as an argument instead of rejecting it; unbindable kinds (funcs, channels, `unsafe.Pointer`, complex numbers) are still rejected [#313](https://github.com/doug-martin/goqu/issues/313), [#316](https://github.com/doug-martin/goqu/pull/316) - [@yz89122](https://github.com/yz89122)
* [FIX] Treat typed nil pointers and invalid `driver.Valuer`s (`sql.NullInt32{}`, ...) as `NULL` in `Eq`/`Neq`/`Is`/`IsNot` and `Ex` [#272](https://github.com/doug-martin/goqu/issues/272), [#438](https://github.com/doug-martin/goqu/issues/438), [#476](https://github.com/doug-martin/goqu/pull/476) - [@funkyshu](https://github.com/funkyshu)
* [FIX] SQL Server: return an error for `ORDER BY` on `DELETE` (new dialect option `ErrorOnUnsupportedOrderByOnDelete`) [#472](https://github.com/doug-martin/goqu/pull/472) - [@funkyshu](https://github.com/funkyshu)
* [FIX] SQL Server: emit `DISTINCT` before `TOP` [#242](https://github.com/doug-martin/goqu/pull/242), [#475](https://github.com/doug-martin/goqu/pull/475) - [@CKehToh](https://github.com/CKehToh), [@funkyshu](https://github.com/funkyshu)
* [FIX] SQLite3: generate `DELETE FROM` for `Truncate` [#330](https://github.com/doug-martin/goqu/pull/330), [#474](https://github.com/doug-martin/goqu/pull/474) - [@darh](https://github.com/darh), [@funkyshu](https://github.com/funkyshu)
* [FIX] MySQL 8: enable `WITH` and `WITH RECURSIVE` common table expressions [#362](https://github.com/doug-martin/goqu/issues/362), [#400](https://github.com/doug-martin/goqu/pull/400), [#473](https://github.com/doug-martin/goqu/pull/473) - [@funkyshu](https://github.com/funkyshu)
* [FIX] Do not wrap a single subselect in double parentheses [#352](https://github.com/doug-martin/goqu/issues/352), [#357](https://github.com/doug-martin/goqu/pull/357) - [@nic11](https://github.com/nic11)
* [DOCS] Fix the `ScanStructContext` description and a typo about placeholders [#424](https://github.com/doug-martin/goqu/pull/424), [#449](https://github.com/doug-martin/goqu/pull/449) - [@shortcuts](https://github.com/shortcuts), [@j0shiJ](https://github.com/j0shiJ)
* [CHORE] Update GitHub Actions dependencies [#465](https://github.com/doug-martin/goqu/pull/465)

## Behavior changes to be aware of

* Comparing a column to a typed null (`(*int)(nil)`, `sql.NullInt32{}` that is not valid, ...) with `Eq`/`Neq`/`Ex` now generates `IS NULL` / `IS NOT NULL` instead of `= NULL` / `!= NULL`. Queries that previously matched no rows will now match `NULL` rows.
* SQL Server `Delete().Order(...)` now returns an error at build time instead of generating SQL the server rejects.
* SQL Server `SELECT DISTINCT ... TOP (n)` is now generated as `SELECT DISTINCT TOP (n) ...`.
* SQLite3 `Truncate()` now generates `DELETE FROM <table>`.
* `goqu.DoNothing()` and `exp.NewDoNothingConflictExpression()` now return `exp.ConflictNothingExpression` (a superset of `exp.ConflictExpression`). Code that reassigns the result to a different conflict type through type inference (`c := goqu.DoNothing(); c = goqu.DoUpdate(...)`) needs an explicit `exp.ConflictExpression` declaration.
* `SetCols` on `DoNothing` for SQLite3 emits a plain `INSERT ... ON CONFLICT (...) DO NOTHING` (not `INSERT OR IGNORE`) so that only the targeted constraint is ignored.

# v9.20.0

**NOTE** The minimum supported Go version is now **1.26** (previously 1.19). See [Supported Go versions](./README.md#supported-go-versions).

* [BREAKING] Raise the minimum Go version to 1.26 [#468](https://github.com/doug-martin/goqu/pull/468) - [@funkyshu](https://github.com/funkyshu)
* [FEATURE] SQL Server: support `Limit` on `Delete` via `DELETE TOP (n)` [#456](https://github.com/doug-martin/goqu/pull/456) - [@saurabh-prakash](https://github.com/saurabh-prakash)
* [FEATURE] Support `uint` values, including `uint64` values above `math.MaxInt64` [#421](https://github.com/doug-martin/goqu/pull/421) - [@walnuts1018](https://github.com/walnuts1018)
* [FEATURE] Allow `[]exp.UpdateExpression` as `Set` values [#389](https://github.com/doug-martin/goqu/pull/389) - [@kvii](https://github.com/kvii)
* [FIX] MySQL: do not add an extra `IGNORE` to `INSERT ... ON DUPLICATE KEY UPDATE` [#435](https://github.com/doug-martin/goqu/pull/435) - [@autowp](https://github.com/autowp)
* [FIX] SQLite3: support `UPDATE ... FROM` [#441](https://github.com/doug-martin/goqu/issues/441), [#457](https://github.com/doug-martin/goqu/pull/457) - [@yousefissa](https://github.com/yousefissa)
* [FIX] SQLite3: support conflict update `WHERE` (`OnConflictUpdateWhere`) [#440](https://github.com/doug-martin/goqu/pull/440) - [@loganintech](https://github.com/loganintech)
* [FIX] SQLite3: allow `RETURNING` clauses [#359](https://github.com/doug-martin/goqu/pull/359) - [@abdusco](https://github.com/abdusco)
* [FIX] Typo in the column list expression builder [#376](https://github.com/doug-martin/goqu/pull/376) - [@philippfrank](https://github.com/philippfrank)
* [DOCS] Fix `Select` docstring [#354](https://github.com/doug-martin/goqu/pull/354) - [@nic11](https://github.com/nic11)
* [CHORE] Clear all open `golang.org/x/crypto` security alerts: the SQL Server driver used by the tests is now `github.com/microsoft/go-mssqldb` [#461](https://github.com/doug-martin/goqu/pull/461), [#469](https://github.com/doug-martin/goqu/pull/469)
* [CHORE] Update dependencies, CI workflows (Compose v2, golangci-lint v2, Codecov v5) and add Dependabot [#386](https://github.com/doug-martin/goqu/pull/386), [#459](https://github.com/doug-martin/goqu/pull/459), [#462](https://github.com/doug-martin/goqu/pull/462), [#468](https://github.com/doug-martin/goqu/pull/468) - [@funkyshu](https://github.com/funkyshu)
* [CHORE] Remove the vendored dependencies [#463](https://github.com/doug-martin/goqu/pull/463)

## Behavior changes to be aware of

* SQL Server `DELETE ... LIMIT n` now generates `DELETE TOP (n) FROM ...`.
* MySQL `INSERT ... ON DUPLICATE KEY UPDATE` no longer generates `INSERT IGNORE ... ON DUPLICATE KEY UPDATE`. `INSERT IGNORE` is still used for `DO NOTHING`.
* `uint64` values greater than `math.MaxInt64` are now encoded (as `uint64` in prepared statements) rather than rejected. Values that fit in an `int64` keep being sent as `int64`.

# v9.19.0
* [FEATURE] Add support for omitempty and omitnil struct tags [#309](https://github.com/doug-martin/goqu/pull/309) - [@randallmlough](https://github.com/randallmlough)

# v9.18.0
* [FEATURE] Add support for aliasing insert datasets to support upsert alias [#306](https://github.com/doug-martin/goqu/pull/306) - [@XIELongDragon](https://github.com/XIELongDragon)
* [FEATURE] Add support for aliasing BooleanExpressions [#307](https://github.com/doug-martin/goqu/pull/307) - [@XIELongDragon](https://github.com/XIELongDragon)

# v9.17.0
* [FEATURE] Add support bitwise operations [#303](https://github.com/doug-martin/goqu/pull/303) - [@XIELongDragon](https://github.com/XIELongDragon)
* [FEATURE] Add support for specifying tables to be locked in ForUpdate, ForNoKeyUpdate, ForKeyShare, ForShare [#299](https://github.com/doug-martin/goqu/pull/299) - [@jbub](https://github.com/jbub)

# v9.16.0
* [FEATURE] Allow ordering by case expression [#282](https://github.com/doug-martin/goqu/issues/282), [#292](https://github.com/doug-martin/goqu/pull/292)

# v9.15.1
* [FIX] Field indexes in the columnMap getting overridden [290](https://github.com/doug-martin/goqu/issues/290), [#291](https://github.com/doug-martin/goqu/pull/291)

# v9.15.0
* [FEATURE] Add SetDefaultPrepared which controls query value interpolation [#288](https://github.com/doug-martin/goqu/pull/288) - [@Deiz](https://github.com/Deiz)

# v9.14.0
* [FEATURE] GroupByAppend to the SelectDataset and SelectClauses [#276](https://github.com/doug-martin/goqu/pull/276), [#287](https://github.com/doug-martin/goqu/pull/287) - [@ashishkf](https://github.com/ashishkf)
* [FEATURE] Allow untagged fields to be ignored [#285](https://github.com/doug-martin/goqu/pull/285) - [@Deiz](https://github.com/Deiz)
* [FIX] Nil valuer [#277](https://github.com/doug-martin/goqu/pull/277) - [@benzolium](https://github.com/benzolium), [@Diggs](https://github.com/Diggs)
* [FIX] Fix old import URL in doc comments [#286](https://github.com/doug-martin/goqu/pull/286) - [@maito1201](https://github.com/maito1201)

# v9.13.0

* [ADDED] ScanStructs, ScanVals to Scanner interface [#273](https://github.com/doug-martin/goqu/issues/273) - [@vlanse](https://github.com/vlanse)

# v9.12.0

* Update golangci-lint and updates for all associated linters
* Update dependencies
  * github.com/DATA-DOG/go-sqlmock  v1.3.3 -> v1.5.0
  * github.com/denisenkom/go-mssqldb v0.0.0-20200206145737-bbfc9a55622e -> v0.10.0
  * github.com/go-sql-driver/mysql v1.4.1 -> v1.6.0
  * github.com/lib/pq v1.2.0 -> v1.10.1
  * github.com/mattn/go-sqlite3 v1.11.0 -> v1.14.7
  * github.com/stretchr/testify -> v1.4.0 -> v1.7.0
  

# v9.11.1

* [FIXED] Avoid mutation of join slice for separate datasets when joins slice capacity is not yet reached [#261](https://github.com/doug-martin/goqu/pull/261) - [@fhaifler](https://github.com/fhaifler)

# v9.11.0

* [FIXED] Use valid 'IS' operator for sqlserver dialect [#240](https://github.com/doug-martin/goqu/pull/240), [#239](https://github.com/doug-martin/goqu/pull/229)  - [@vlanse](https://github.com/vlanse)
* [ADDED] Implement Orderable interface for SQL Functions [#251](https://github.com/doug-martin/goqu/pull/251)  - [@GlebBeloded](https://github.com/GlebBeloded)
* [ADDED] Support for table hint in multi-table MySQL DELETE queries [#252](https://github.com/doug-martin/goqu/pull/252) - [@vlanse](https://github.com/vlanse)

# v9.10.0

* [FIXED] SELECT inherits dialect from INSERT in INSERT FROM SELECT.  [#229](https://github.com/doug-martin/goqu/pull/229), [#223](https://github.com/doug-martin/goqu/issues/223) - [@vlanse](https://github.com/vlanse)
* [FIXED] SQLServer dialect: support prepared statements with TOP.  [#230](https://github.com/doug-martin/goqu/pull/230), [#225](https://github.com/doug-martin/goqu/issues/225) - [@vlanse](https://github.com/vlanse)
* [ADDED] IsPrepared to SQLExpression interface.  [#231](https://github.com/doug-martin/goqu/pull/231) - [@vlanse](https://github.com/vlanse)

# v9.9.0

* [FIXED] SQLite do not add FOR UPDATE in SELECT. [#218](https://github.com/doug-martin/goqu/pull/218) - [@vlanse](https://github.com/vlanse)
* [ADDED] Support for INSERT ON CONFLICT in SQLite. [#218](https://github.com/doug-martin/goqu/pull/218) - [@vlanse](https://github.com/vlanse)

# v9.8.0

* [ADDED] Support for ANY and ALL operators. [#196](https://github.com/doug-martin/goqu/issues/196)
* [ADDED] Support for CASE statements [#193](https://github.com/doug-martin/goqu/issues/193) 
* [ADDED] Support for getting column identifiers from AliasExpressions. [#203](https://github.com/doug-martin/goqu/issues/203)  

# v9.7.1

* Fix all formatting for golangci-lint
* Move to golangci-lint github action

# v9.7.0

* [ADDED] Support for sqlserver dialect [#197](https://github.com/doug-martin/goqu/issues/197),[#205](https://github.com/doug-martin/goqu/issues/205) - [@vlanse](https://github.com/vlanse)

# v9.6.0

* [ADDED] Support for Lateral queries [#182](https://github.com/doug-martin/goqu/issues/182)

# v9.5.1

* [FIXED] WITH clause without a RETURNING clause will panic [#177](https://github.com/doug-martin/goqu/issues/177)
* [FIXED] SQlite dialect escapes single quotes wrong, leads to SQL syntax error [#178](https://github.com/doug-martin/goqu/issues/178)
* [FIXED] Fix ReturnsColumns() nil pointer panic [#181](https://github.com/doug-martin/goqu/issues/181) - [@yeaha](https://github.com/yeaha)
* [FIXED] SelectDataset From with Error [#183](https://github.com/doug-martin/goqu/issues/183)
* [FIXED] Unable to execute union with order by expression [#185](https://github.com/doug-martin/goqu/issues/185)

# v9.5.0

* [ADDED] Ability to use regexp like, ilike, notlike, and notilike without a regexp [#172](https://github.com/doug-martin/goqu/issues/172)

# v9.4.0

* [ADDED] Ability to scan into struct fields from multiple tables [#160](https://github.com/doug-martin/goqu/issues/160)

# v9.3.0

* [ADDED] Using Update, Insert, or Delete datasets in sub selects and CTEs [#164](https://github.com/doug-martin/goqu/issues/164)

# v9.2.0

* [ADDED] exec.Scanner: New exposed scanner supports iterative scanning [#157](https://github.com/doug-martin/goqu/pull/157) - [@akarl](https://github.com/akarl)

# v9.1.0

* [FIXED] ExampleDoUpdate does't work in postgres [#156](https://github.com/doug-martin/goqu/issues/156)
* [FIXED] Issue with timezone being lost [#163](https://github.com/doug-martin/goqu/issues/163) 

# v9.0.1

* [FIXED] Issue where `NULL`, `TRUE` and `FALSE` are interpolated when using an `IS` clause. [#165](https://github.com/doug-martin/goqu/issues/165)
    
# v9.0.0

* Changed `NULL`, `TRUE`, `FALSE` to not be interpolated when creating prepared statements. [#132](https://github.com/doug-martin/goqu/pull/132), [#158](https://github.com/doug-martin/goqu/pull/158) - [@marshallmcmullen](https://github.com/marshallmcmullen)
* Updated dependencies
    * `github.com/lib/pq v1.1.1 -> v1.2.0`
    * `github.com/mattn/go-sqlite3 v1.10.0 -> v1.11.0`
    * `github.com/stretchr/testify v1.3.0 -> v1.4.0`

## v8.6.0

* [ADDED] `SetError()` and `Error()` to all datasets. [#152](https://github.com/doug-martin/goqu/pull/152) and [#150](https://github.com/doug-martin/goqu/pull/150) - [@marshallmcmullen](https://github.com/marshallmcmullen)

## v8.5.0

* [ADDED] Window Function support [#128](https://github.com/doug-martin/goqu/issues/128) - [@Xuyuanp](https://github.com/Xuyuanp)

## v8.4.1

* [FIXED] Returning func be able to handle nil [#140](https://github.com/doug-martin/goqu/issues/140)

## v8.4.0

* Created new `sqlgen` module to encapsulate sql generation
    * Broke SQLDialect inti new SQL generators for each statement type.
* Test refactor
    * Moved to a test case pattern to allow for quickly adding new test cases.
    
## v8.3.2

* [FIXED] Data race during query factory initialization [#133](https://github.com/doug-martin/goqu/issues/133) and [#136](https://github.com/doug-martin/goqu/issues/136) - [@o1egl](https://github.com/o1egl)    

## v8.3.1

* [FIXED] InsertDataset.WithDialect return old dataset [#126](https://github.com/doug-martin/goqu/issues/126) - [@chen56](https://github.com/chen56)
* Test clean up and more testing pattern consistency
    * Changed to use assertion methods off of suite
    * Updated Equals assertions to have expected output first 
* Increase overall test coverage.

## v8.3.0

* [Added] Support for `DISTINCT ON` clauses [#119](https://github.com/doug-martin/goqu/issues/119)

## v8.2.2

* [FIX] Scanner errors on pointers to primitive values [#122](https://github.com/doug-martin/goqu/issues/122)

## v8.2.1

* [FIX] Return an error when an empty identifier is encountered [#115](https://github.com/doug-martin/goqu/issues/115)

## v8.2.0

* [FIX] Fix reflection errors related to nil pointers and unexported fields [#118](https://github.com/doug-martin/goqu/issues/118)
    * Unexported fields are ignored when creating a columnMap
    * Nil embedded pointers will no longer cause a panic
    * Fields on nil embedded pointers will be ignored when creating update or insert statements.
* [ADDED] You can now ingore embedded structs and their fields by using `db:"-"` tag on the embedded struct.

## v8.1.0

* [ADDED] Support column DEFAULT when inserting/updating via struct [#27](https://github.com/doug-martin/goqu/issues/27)

## v8.0.1

* [ADDED] Multi table update support for `mysql` and `postgres` [#60](https://github.com/doug-martin/goqu/issues/60)
* [ADDED] `goqu.V` so values can be used on the LHS of expressions [#104](https://github.com/doug-martin/goqu/issues/104)

## v8.0.0

A major change the the API was made in `v8` to seperate concerns between the different SQL statement types. 

**Why the change?**

1. There were feature requests that could not be cleanly implemented with everything in a single dataset. 
2. Too much functionality was encapsulated in a single datastructure.
    * It was unclear what methods could be used for each SQL statement type.
    * Changing a feature for one statement type had the possiblity of breaking another statement type.
    * Test coverage was decent but was almost solely concerned about SELECT statements, breaking them up allowed for focused testing on each statement type.
    * Most the SQL generation methods (`ToInsertSQL`, `ToUpdateSQL` etc.) took arguments which lead to an ugly API that was not uniform for each statement type, and proved to be inflexible.

**What Changed**

There are now five dataset types, `SelectDataset`, `InsertDataset`, `UpdateDataset`, `DeleteDataset` and `TruncateDataset`

Each dataset type has its own entry point.

* `goqu.From`, `Database#From`, `DialectWrapper#From` - Create SELECT
* `goqu.Insert`, `Database#Insert`, `DialectWrapper#Insert` - Create INSERT
* `goqu.Update`, `Database#db.Update`, `DialectWrapper#Update` - Create UPDATE
* `goqu.Delete`, `Database#Delete`, `DialectWrapper#Delete` - Create DELETE
* `goqu.Truncate`, `Database#Truncate`, `DialectWrapper#Truncate` - Create TRUNCATE
  
`ToInsertSQL`, `ToUpdateSQL`, `ToDeleteSQL`, and `ToTruncateSQL` (and variations of them) methods have been removed from the `SelectDataset`. Instead use the `ToSQL` methods on each dataset type.

Each dataset type will have an `Executor` and `ToSQL` method so a common interface can be created for each type.


## v7.4.0

* [FIXED] literalTime use t.UTC() , This behavior is different from the original sql.DB [#106](https://github.com/doug-martin/goqu/issues/106) - [chen56](https://github.com/chen56)
* [ADDED] Add new method WithTx for Database [#108](https://github.com/doug-martin/goqu/issues/108) - [Xuyuanp](https://github.com/Xuyuanp)

## v7.3.1

* [ADDED] Exposed `goqu.NewTx` to allow creating a goqu tx directly from a `sql.Tx` instead of using `goqu.Database#Begin` [#95](https://github.com/doug-martin/goqu/issues/95)
* [ADDED] `goqu.Database.BeginTx` [#98](https://github.com/doug-martin/goqu/issues/98)

## v7.3.0

* [ADDED] UPDATE and INSERT should use struct Field name if db tag is not specified [#57](https://github.com/doug-martin/goqu/issues/57)
* [CHANGE] Changed goqu.Database to accept a SQLDatabase interface to allow using goqu.Database with other libraries such as `sqlx` [#95](https://github.com/doug-martin/goqu/issues/95)

## v7.2.0

* [FIXED] Sqlite3 does not accept SELECT * UNION (SELECT *) [#79](https://github.com/doug-martin/goqu/issues/79)
* [FIXED] Where(Ex{}) causes panics [mysql] [#49](https://github.com/doug-martin/goqu/issues/49)
* [ADDED] Support for OrderPrepend [#61](https://github.com/doug-martin/goqu/issues/61)
* [DOCS] Added new section about loading a dialect and using it to build SQL [#44](https://github.com/doug-martin/goqu/issues/44)

## v7.1.0

* [FIXED] Embedded pointers with property names that duplicate parent struct properties. [#23](https://github.com/doug-martin/goqu/issues/23)
* [FIXED] Can't scan values using []byte or []string [#90](https://github.com/doug-martin/goqu/issues/90)
    * When a slice that is `*sql.RawBytes`, `*[]byte` or `sql.Scanner` no errors will be returned. 

## v7.0.1

* Fix issue where structs with pointer fields where not set properly [#86](https://github.com/doug-martin/goqu/pull/86) and [#89](https://github.com/doug-martin/goqu/pull/89) - [@efureev](https://github.com/efureev)

## v7.0.0

**Linting**
* Add linting checks and fixed errors 
    * Renamed all snake_case variables to be camelCase.     
    * Fixed examples to always map to a defined method
* Renamed `adapters` to `dialect` to more closely match their intended purpose.

**API Changes**
* Updated all sql generations methods to from `Sql` to `SQL`
    * `ToSql` -> `ToSQL`
    * `ToInsertSql` -> `ToInsertSQL`
    * `ToUpdateSql` -> `ToUpdateSQL`
    * `ToDeleteSql` -> `ToDeleteSQL`
    * `ToTruncateSql` -> `ToTruncateSQL`
* Abstracted out `dialect_options` from the adapter to make the dialect self contained.
    * This also removed the dataset<->adapter co dependency making the dialect self contained.
* Refactored the `goqu.I` method.
    * Added new `goqu.S`, `goqu.T` and `goqu.C` methods to clarify why type of identifier you are using.
    * `goqu.I` should only be used when you have a qualified identifier (e.g. `goqu.I("my_schema.my_table.my_col")
* Added new `goqu.Dialect` method to make using `goqu` as an SQL builder easier.

**Internal Changes**
* Pulled expressions into their own package
    * Broke up expressions.go into multiple files to make working with and defining them easier.
    * Moved the user facing methods into the main `goqu` to keep the same API as before.
* Added more examples
* Moved non-user facing structs and interfaces to internal modules to clean up API.
* Increased test coverage.
 

## v6.1.0

* Handle nil *time.Time Literal [#73](https://github.com/doug-martin/goqu/pull/73) and [#52](https://github.com/doug-martin/goqu/pull/52) - [@RoarkeRandall](https://github.com/RoarkeRandall) and [@quetz](https://github.com/quetz)
* Add ability to change column rename function [#66](https://github.com/doug-martin/goqu/pull/66) - [@blainehansen](https://github.com/blainehansen)

## v6.0.0

* Updated go support to `1.10`, `1.11` and `1.12`
* Change testify dependency from c2fo/testify back to stretchr/testify.
* Add support for "FOR UPDATE" and "SKIP LOCKED" [#62](https://github.com/doug-martin/goqu/pull/62) - [@btubbs](https://github.com/btubbs)
* Changed to use go modules

## v5.0.0

* Drop go 1.6 support, supported versions are `1.8`, `1.9` and latest
* Add context support [#64](https://github.com/doug-martin/goqu/pull/64) - [@cmoad](https://github.com/cmoad)

## v4.2.0

* Add support for ON CONFLICT when using a dataset [#55](https://github.com/doug-martin/goqu/pull/55) - [@bobrnor](https://github.com/bobrnor)

## v4.1.0

* Support for defining WITH clauses for Common Table Expressions (CTE) [#39](https://github.com/doug-martin/goqu/pull/39) - [@Oscil8](https://github.com/Oscil8)

## v4.0

* Prepared(true) issues when using IS NULL comparisson operation [#33](https://github.com/doug-martin/goqu/pull/33) - [@danielfbm](https://github.com/danielfbm)

## v3.3

* Add `upsert` support via `InsertIgnore` and `InsertConflict` methods - [#25](https://github.com/doug-martin/goqu/pull/28) - [@aheuermann](https://github.com/aheuermann)
* Adding vendor dependencies and updating tests to run in docker containers [#29](https://github.com/doug-martin/goqu/pull/29) - [@aheuermann](https://github.com/aheuermann)

## v3.2

* Add range clauses ([NOT] BETWEEN) support - [#25](https://github.com/doug-martin/goqu/pull/25) - [@denisvm](https://github.com/denisvm)
* Readmefix [#26](https://github.com/doug-martin/goqu/pull/26) - [@tiagopotencia](https://github.com/tiagopotencia)

## v3.1.3

* Bugfix for chained Where() [#20](https://github.com/doug-martin/goqu/pull/20) - [@Emreu](https://github.com/Emreu)


## v3.1.2

* Fixing ScanStruct issue with embedded pointers in crud_exec [#20](https://github.com/doug-martin/goqu/pull/20) - [@ruzz311](https://github.com/ruzz311)

## v3.1.1

* Fixing race condition with struct_map_cache in crud_exec [#18](https://github.com/doug-martin/goqu/pull/18) - [@andymoon](https://github.com/andymoon), [@aheuermann](https://github.com/aheuermann)

## v3.1.0

* Version 3.1 [#14](https://github.com/doug-martin/goqu/pull/14) - [@andymoon](https://github.com/andymoon)
    * Fix an issue with a nil pointer access on the inserts and updates.
    * Allowing ScanStructs to take a struct with an embedded pointer to a struct.
    * Change to check if struct is Anonymous when recursing through an embedded struct.
    * Updated to use the latest version of github.com/DATA-DOG/go-sqlmock.

## v3.0.1

* Add literal bytes and update to c2fo testify [#15](https://github.com/doug-martin/goqu/pull/15) - [@TechnotronicOz](https://github.com/TechnotronicOz)

## v3.0.0

* Added support for embedded structs when inserting or updating. [#13](https://github.com/doug-martin/goqu/pull/13) - [@andymoon](https://github.com/andymoon)

## v2.0.3

* Fixed issue with transient columns and the auto select of columns.

## v2.0.2

* Changed references to "github.com/doug-martin/goqu" to "gopkg.in/doug-martin/goqu.v2"

## v2.0.1

* Fixed issue when `ScanStruct(s)` was used with `SelectDistinct` and caused a panic.

## v2.0.0

* When scanning a struct or slice of structs, the struct(s) will be parsed for the column names to select. [#9](https://github.com/doug-martin/goqu/pull/9) - [@technotronicoz](https://github.com/TechnotronicOz)

## v1.0.0

* You can now passed an IdentiferExpression to `As` [#8](https://github.com/doug-martin/goqu/pull/8) - [@croachrose](https://github.com/croachrose)
* Added info about installation through [gopkg.in](http://labix.org/gopkg.in)

## v0.3.1

* Fixed issue setting Logger when starting a new transaction.

## v0.3.0

* Changed sql generation methods to use a common naming convention. `To(Sql|Insert|Update|Delete)`
   * Also changed to have common return values `string, []interface{}, error)`
* Added `Dataset.Prepared` which allows a user to specify whether or not SQL should be interpolated. [#7](https://github.com/doug-martin/goqu/issues/7)
* Updated Docs
    * More examples
* Increased test coverage.

## v0.2.0

* Changed `CrudExec` to not wrap driver errors in a GoquError [#2](https://github.com/doug-martin/goqu/issues/2)
* Added ability to use a dataset in an `Ex` map or `Eq` expression without having to use `In` [#3](https://github.com/doug-martin/goqu/issues/3)
   * `db.From("test").Where(goqu.Ex{"a": db.From("test").Select("b")})`
* Updated readme with links to [`DefaultAdapter`](https://godoc.org/github.com/doug-martin/goqu#DefaultAdapter)

## v0.1.1

* Added SQLite3 adapter [#1](https://github.com/doug-martin/goqu/pull/1) - [@mattn](https://github.com/mattn)

## v0.1.0

* Added:
    * [`Ex`](https://godoc.org/github.com/doug-martin/goqu#Ex)
    * [`ExOr`](https://godoc.org/github.com/doug-martin/goqu#ExOr)
    * [`Op`](https://godoc.org/github.com/doug-martin/goqu#Op)
* More tests and examples
* Added CONTRIBUTING.md
* Added LICENSE information
* Removed godoc introduction in favor of just maintaining the README.

## v0.0.2

* Fixed issue with goqu.New not returning a pointer to a Database

## v0.0.1

* Initial release
