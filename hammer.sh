#!/bin/bash

set -eu

make build

for i in {1..10}
do
  echo "try test count" $i
  go clean -testcache
  go test -v ./...
done
