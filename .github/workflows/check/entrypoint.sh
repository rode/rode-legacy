#!/bin/sh 
set -e
if [ -z "$(gofmt -l .)" ]; then echo "Format OK"; else echo "Format Fail. Run \"go fmt ./...\""; exit 1; fi
go test -cover -tags unit ./...
