#!/bin/bash

# sudo apt install build-essential -y
# go build -o SQLiteQueryServer --tags linux -ldflags "-extldflags -static"

VERSION="$(git describe --tags $(git rev-list --tags --max-count=1))"

go get -u -v github.com/karalabe/xgo
xgo --targets windows/amd64 --out SQLiteQueryServer-"$VERSION" github.com/assafmo/SQLiteQueryServer
xgo --targets linux/amd64   --out SQLiteQueryServer-"$VERSION" --tags linux --ldflags "-extldflags -static"  github.com/assafmo/SQLiteQueryServer
