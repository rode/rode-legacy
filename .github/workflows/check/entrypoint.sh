#!/bin/sh 
set -e
go test -cover -tags unit ./...
golangci-lint run
