#!/usr/bin/env bash

set -eux

export GO111MODULE=on

cover_pkgs=$(go list ./... | grep -v /cmd | grep -v /vendor | grep -v /test | tr "\n" ",")

# install ginkgo cli
go install github.com/onsi/ginkgo/ginkgo

ginkgo \
  --progress --noColor --v \
  --nodes=1 \
  -timeout=20m \
  -cover \
  -covermode atomic \
  -coverprofile coverage.out \
  -coverpkg "$cover_pkgs" \
  -outputdir "$(pwd)" \
  ./test/e2e

tail -n +2 coverage.out >> coverage.txt || exit 255
rm coverage.out

CIRCLECI=${CIRCLECI:-}
if [[ ! -z "$CIRCLECI" ]]; then
  mkdir -p ./test/result/
  find . -type f -regex "./test/e2e/.*unit-test.xml" -exec cp {} ./test/result/ \;
  find . -type f -regex "./test/e2e/.*unit-test.xml" -exec rm {} +;
  ls -al ./test/result/;
fi