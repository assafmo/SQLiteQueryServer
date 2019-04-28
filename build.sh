#!/bin/bash

# build into ./build/

set -e
set -v

go test -race ./...

rm -rf build
mkdir -p build

VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))

go get -u -v github.com/karalabe/xgo

xgo --targets windows/amd64 --dest build --out SQLiteQueryServer-"${VERSION}" .
xgo --targets linux/amd64   --dest build --out SQLiteQueryServer-"${VERSION}" --tags linux --ldflags "-extldflags -static"  .

(
    cd build
    find -type f | 
    parallel --bar 'zip "$(echo "{}" | sed "s/.exe//").zip" "{}"'
)
