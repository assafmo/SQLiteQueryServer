# SQLiteQueryServer

Bulk query SQLite database over the network.  
Way faster than [SQLiteProxy](https://github.com/assafmo/SQLiteProxy)!

[![CircleCI](https://circleci.com/gh/assafmo/SQLiteQueryServer.svg?style=shield&circle-token=cda4af2f2b6cc0035287b25086c596d2ef44d9ce)](https://circleci.com/gh/assafmo/SQLiteQueryServer)
[![Coverage Status](https://coveralls.io/repos/github/assafmo/SQLiteQueryServer/badge.svg?branch=master)](https://coveralls.io/github/assafmo/SQLiteQueryServer?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/assafmo/SQLiteQueryServer)](https://goreportcard.com/report/github.com/assafmo/SQLiteQueryServer)

# Installation

- Download a precompiled binary from https://github.com/assafmo/SQLiteQueryServer/releases
- Or use `go get`:

  ```bash
  go get -u github.com/assafmo/SQLiteQueryServer
  ```

  This package uses `github.com/mattn/go-sqlite3`. Compilation errors might be resolved by reading https://github.com/mattn/go-sqlite3#compilation.

- Or use Ubuntu PPA:

  ```bash
  curl -SsL https://assafmo.github.io/ppa/ubuntu/KEY.gpg | sudo apt-key add -
  sudo curl -SsL -o /etc/apt/sources.list.d/assafmo.list https://assafmo.github.io/ppa/ubuntu/assafmo.list
  sudo apt update
  sudo apt install sqlitequeryserver
  ```

# Usage

```
Usage of SQLiteQueryServer:
  -db string
        Filesystem path of the SQLite database
  -port uint
        HTTP port to listen on (default 80)
  -query string
        SQL query to prepare for
```

Note: SQLiteQueryServer is optimized for the SELECT command. Other commands such as INSERT, UPDATE, DELETE, CREATE, etc might be slow because SQLiteQueryServer doesn't use transactions (yet). Also, the response format and error messages from these commands may be odd or unexpected.

# Examples

## Creating a server

```bash
SQLiteQueryServer --db "$DB_PATH" --query "$PARAMETERIZED_SQL_QUERY" --port "$PORT"
```

```bash
SQLiteQueryServer --db ./test_db/ip_dns.db --query "SELECT * FROM ip_dns WHERE dns = ?" --port 8080
```

This will expose the `./test_db/ip_dns.db` database with the query `SELECT * FROM ip_dns WHERE dns = ?` on port `8080`.  
Requests will need to provide the query parameters.

## Querying the server

```bash
echo -e "github.com\none.one.one.one\ngoogle-public-dns-a.google.com" | curl "http://localhost:8080/query" --data-binary @-
```

```bash
echo -e "$QUERY1_PARAM1,$QUERY1_PARAM2\n$QUERY2_PARAM1,$QUERY2_PARAM2" | curl "http://$ADDRESS:$PORT/query" --data-binary @-
```

```bash
curl "http://$ADDRESS:$PORT/query" -d "$PARAM_1,$PARAM_2,...,$PARAM_N"
```

- Request must be a HTTP POST to "http://$ADDRESS:$PORT/query".
- Request body must be a valid CSV.
- Request body must not have a CSV header.
- Each request body line is a different query.
- Each param in a line corresponds to a query param (a question mark in the query string).
- Static query (without any query params):
  - The request must be a HTTP GET to "http://$ADDRESS:$PORT/query".
  - The query executes only once.

## Getting a response

```bash
echo -e "github.com\none.one.one.one\ngoogle-public-dns-a.google.com" | curl "http://localhost:8080/query" --data-binary @-
```

```json
[
  {
    "in": ["github.com"],
    "headers": ["ip", "dns"],
    "out": [["192.30.253.112", "github.com"], ["192.30.253.113", "github.com"]]
  },
  {
    "in": ["one.one.one.one"],
    "headers": ["ip", "dns"],
    "out": [["1.1.1.1", "one.one.one.one"]]
  },
  {
    "in": ["google-public-dns-a.google.com"],
    "headers": ["ip", "dns"],
    "out": [["8.8.8.8", "google-public-dns-a.google.com"]]
  }
]
```

- If response status is 200 (OK), response is a JSON array (`Content-Type: application/json`).
- Each element in the array:
  - Is a result of a query
  - Has an "in" field which is an array of the input params (a request body line).
  - Has an "headers" field which is an array of headers of the SQL query result.
  - Has an "out" field which is an array of arrays of results. Each inner array is a result row.
- Element #1 is the result of query #1, Element #2 is the result of query #2, and so forth.
- Static query (without any query params):
  - The response JSON has only one element.

## Static query

```bash
SQLiteQueryServer --db ./test_db/ip_dns.db --query "SELECT * FROM ip_dns" --port 8080
```

```bash
curl "http://localhost:8080/query"
```

```json
[
  {
    "in": [],
    "headers": ["ip", "dns"],
    "out": [
      ["1.1.1.1", "one.one.one.one"],
      ["8.8.8.8", "google-public-dns-a.google.com"],
      ["192.30.253.112", "github.com"],
      ["192.30.253.113", "github.com"]
    ]
  }
]
```
