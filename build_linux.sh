#!/bin/bash

#sudo apt install build-essential -y
go build -o SQLiteQueryServer --tags linux -ldflags "-extldflags -static"