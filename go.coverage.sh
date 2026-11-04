#!/usr/bin/env bash

set -e
echo "" > coverage.txt

go test -coverprofile=coverage.txt -coverpkg=./... $(go list ./... | grep -v /dialect/duckdb)
go test -mod=readonly -coverprofile=coverage.duckdb.txt -coverpkg=./... ./dialect/duckdb/...
