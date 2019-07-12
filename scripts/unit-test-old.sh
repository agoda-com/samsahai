#!/usr/bin/env bash

set -eux

export GO111MODULE=on

echo 'mode: atomic' > coverage.txt
touch ./coverage.out

cover_pkgs=$(go list ./... | grep -v /cmd | grep -v /vendor | grep -v /test | tr "\n" ",")

for pkg in $(go list ./internal/... | grep -v cmd); do
  go test \
    -race \
    -covermode=atomic \
    -coverprofile=coverage.out \
    -coverpkg $cover_pkgs $pkg
  tail -n +2 coverage.out >> coverage.txt || exit 255
  rm coverage.out
done

for pkg in $(go list ./pkg/... | grep -v cmd); do
  go test \
    -race \
    -covermode=atomic \
    -coverprofile=coverage.out \
    -coverpkg $cover_pkgs $pkg
  tail -n +2 coverage.out >> coverage.txt || exit 255
  rm coverage.out
done

CIRCLECI=${CIRCLECI:-}
if [[ ! -z "$CIRCLECI" ]]; then
  mkdir -p ./test/result/
  ls -al ./test/result/
  find . -type f -regex "./.*unit-test.xml" -exec cp {} ./test/result/ \;
  ls -al ./test/result/
  find . -type f -regex "./internal/.*unit-test.xml" -exec rm {} +;
  find . -type f -regex "./pkg/.*unit-test.xml" -exec rm {} +;
  ls -al ./test/result/
fi