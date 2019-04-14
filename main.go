package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var queryStmt *sql.Stmt

var dbPath string
var queryString string
var port uint

var help = `SQLiteQueryServer

`

func init() {
	flag.StringVar(&dbPath, "db", "", "Path to DB")
	flag.StringVar(&queryString, "query", "", "The SQL query")
	flag.UintVar(&port, "port", 80, "Port of the http server")

	flag.Parse()

	var err error
	db, err = sql.Open("sqlite3", fmt.Sprintf("file:%s&mode=rw&cache=shared", dbPath))
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(1)

	queryStmt, err = db.Prepare(queryString)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	http.HandleFunc("/query", query)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	if err != nil {
		panic(err)
	}
}

func query(w http.ResponseWriter, r *http.Request) {

}
