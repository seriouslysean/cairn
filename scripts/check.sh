#!/bin/sh
# check.sh runs every check a change must pass. It prints what fails and exits non-zero.
# staticcheck and go fix -diff need network access the first time, and go fix -diff needs Go 1.26 or later.
set -eu
cd "$(dirname "$0")/.."

fail=0
out=$(gofmt -s -l .)
if [ -n "$out" ]; then
	echo "gofmt -s: these files need formatting:"
	echo "$out"
	fail=1
fi
out=$(go fix -diff ./...)
if [ -n "$out" ]; then
	echo "go fix: run 'go fix ./...':"
	echo "$out"
	fail=1
fi
go vet ./... || fail=1
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 -checks all ./... || fail=1
go test ./... || fail=1
exit $fail
