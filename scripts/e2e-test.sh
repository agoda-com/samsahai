#!/usr/bin/env bash

set -eux

export GO111MODULE=on

GO=${GO:-"go"}

cover_pkgs=$($GO list ./... | grep -v /cmd | grep -v /vendor | grep -v /test | tr "\n" ",")

# install ginkgo cli
eval $GO install github.com/onsi/ginkgo/ginkgo

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

CI=${CI:-}
if [[ ! -z "$CI" ]]; then
  mkdir -p ./test/result/
  find . -type f -regex "./test/e2e/.*unit-test.xml" -exec cp {} ./test/result/ \;
  find . -type f -regex "./test/e2e/.*unit-test.xml" -exec rm {} +;
  ls -al ./test/result/;
fi