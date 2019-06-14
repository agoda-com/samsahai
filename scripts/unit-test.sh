#!/usr/bin/env bash

set -e

echo "" > coverage.txt

for d in $(go list ./internal/... | grep -v cmd); do
    GO111MODULE=on go test -race -v -covermode=atomic \
        -coverprofile=coverage.out $d
    if [ -f coverage.out ]; then
        cat coverage.out >> coverage.txt
        rm coverage.out
    fi
done