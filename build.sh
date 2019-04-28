#!/bin/bash

# build into ./release/

set -e
set -v

go test -race ./...

rm -rf release
mkdir -p release

VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))

go get -u -v github.com/karalabe/xgo

xgo --targets windows/amd64 --dest release --out SQLiteQueryServer-"${VERSION}" .
xgo --targets linux/amd64   --dest release --out SQLiteQueryServer-"${VERSION}" --tags linux --ldflags "-extldflags -static"  .

(
    cd release
    find -type f | 
    parallel --bar 'zip "$(echo "{}" | sed "s/.exe//").zip" "{}"'
)
