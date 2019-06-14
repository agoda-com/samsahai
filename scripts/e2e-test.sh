#!/usr/bin/env bash

set -e

for d in $(go list ./... | grep -v cmd); do
    GO111MODULE=on go test -p 1 -race -v -covermode=atomic -coverprofile=coverage.out $d
    if [ -f coverage.out ]; then
        cat coverage.out >> coverage.txt
        rm coverage.out
    fi
done