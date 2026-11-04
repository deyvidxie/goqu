#!/usr/bin/env bash

set -e
echo "" > coverage.txt

go test -v -race $(go list ./... | grep -v /dialect/duckdb)
go test -v -race -mod=mod ./dialect/duckdb/...
