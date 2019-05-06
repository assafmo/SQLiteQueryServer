#!/bin/bash

# build into ./release/

set -e
set -v

go get -v -u -t -d ./...

go test -race -cover ./...

rm -rf release
mkdir -p release

VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))

go get -u -v github.com/karalabe/xgo

xgo --targets windows/amd64 --dest release --out SQLiteQueryServer-"${VERSION}" .
xgo --targets linux/amd64   --dest release --out SQLiteQueryServer-"${VERSION}" --tags linux --ldflags "-extldflags -static"  .

(
    # zip
    cd release
    find -type f | 
        parallel --bar 'zip "$(echo "{}" | sed "s/.exe//").zip" "{}" && rm -f "{}"'

    # deb
    mkdir -p ./deb/bin
    unzip -o -d ./deb/bin *-linux-amd64.zip
    mv -f ./deb/bin/*-linux-amd64 ./deb/bin/SQLiteQueryServer

    mkdir -p ./deb/DEBIAN
    cat > ./deb/DEBIAN/control <<EOF 
Package: SQLiteQueryServer
Version: $(echo "${VERSION}" | tr -d v)
Priority: optional
Architecture: amd64
Maintainer: Assaf Morami <assaf.morami@gmail.com>
Homepage: https://github.com/assafmo/SQLiteQueryServer
Installed-Size: $(stat --printf="%s" ./deb/bin/SQLiteQueryServer)
Provides: sqlitequeryserver
Conflicts: sqlitequeryserver
Replaces: sqlitequeryserver
Description: Bulk query SQLite database over the network. 
EOF

    dpkg-deb --build ./deb/ .
)
