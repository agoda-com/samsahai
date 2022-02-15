#!/usr/bin/env bash

set -eux

export GO111MODULE=on

GO=${GO:-"go"}

echo 'mode: atomic' > coverage.txt
touch ./coverage.out

cover_pkgs=$($GO list ./... | grep -v /cmd | grep -v /vendor | grep -v /test | tr "\n" ",")

eval $GO test \
  -race \
  -covermode=atomic \
  -coverprofile=coverage.out \
  -coverpkg $cover_pkgs \
  ./internal/...
tail -n +2 coverage.out >> coverage.txt || exit 255
rm coverage.out

eval $GO test \
  -race \
  -covermode=atomic \
  -coverprofile=coverage.out \
  -coverpkg $cover_pkgs \
  ./pkg/...
tail -n +2 coverage.out >> coverage.txt || exit 255
rm coverage.out

eval $GO test \
  -race \
  -covermode=atomic \
  -coverprofile=coverage.out \
  -coverpkg $cover_pkgs \
  ./api/...
tail -n +2 coverage.out >> coverage.txt || exit 255
rm coverage.out

CI=${CI:-}
if [[ ! -z "$CI" ]]; then
  mkdir -p ./test/result/
  ls -al ./test/result/
  find . -type f -regex "./.*unit-test.xml" -exec cp {} ./test/result/ \;
  ls -al ./test/result/
  find . -type f -regex "./internal/.*unit-test.xml" -exec rm {} +;
  find . -type f -regex "./pkg/.*unit-test.xml" -exec rm {} +;
  find . -type f -regex "./api/.*unit-test.xml" -exec rm {} +;
  ls -al ./test/result/
fi