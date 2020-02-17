#!/bin/sh 
set -e
go test -cover -tag=unit ./...
golangci-lint run
