package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var queryStmt *sql.Stmt

var dbPath string
var queryString string
var serverPort uint

var dbSchema string

var helpMessege string

func init() {
	flag.StringVar(&dbPath, "db", "", "Path to DB")
	flag.StringVar(&queryString, "query", "", "The SQL query")
	flag.UintVar(&serverPort, "port", 80, "Port of the http server")

	flag.Parse()

	var err error
	db, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rw&cache=shared", dbPath))
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(1)

	queryStmt, err = db.Prepare(queryString)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(`DB:
	%s
`, dbPath)
	fmt.Printf(`Port:
	%d
`, serverPort)

	helpMessege += fmt.Sprintf(`Query:
	%s
`, queryString)
	helpMessege += fmt.Sprintf(`Params count (question marks):
	%d
`, strings.Count(queryString, "?"))
	helpMessege += fmt.Sprintf(`Usage:
	curl "http://$ADDRESS:%d/query" -d "$PARAM_1,$PARAM_2,...,$PARAM_N"

	- Request must be a HTTP POST to /query
	- Request body must be a valid CSV
	- Request body must not have a CSV header
	- Each request body line is a different query
	- Each request body param corresponds to a query param (a question mark in the query string)
`, serverPort)

	fmt.Printf(helpMessege)
}

func main() {
	http.HandleFunc("/query", query)
	err := http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil)

	if err != nil {
		log.Fatal(err)
	}
}

func query(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/query" {
		http.Error(w, helpMessege, http.StatusNotFound)
		return
	}
	if r.Method != "POST" {
		http.Error(w, helpMessege, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	reqReader := csv.NewReader(bufio.NewReader(r.Body))
	for {
		csvLine, err := reqReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			http.Error(w,
				fmt.Sprintf("Error reading request body: %v\n\n%s", err, helpMessege), http.StatusInternalServerError)
			return
		}

		queryParams := make([]interface{}, len(csvLine))
		for i := range csvLine {
			queryParams[i] = csvLine[i]
		}

		rows, err := queryStmt.Query(queryParams...)
		if err != nil {
			http.Error(w,
				fmt.Sprintf("Error executing query for params %v: %v\n\n%s", csvLine, err, helpMessege), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var row []interface{}
			err = rows.Scan(&row)
			if err != nil {
				http.Error(w,
					fmt.Sprintf("Error reading query results for params %v: %v\n\n%s", csvLine, err, helpMessege), http.StatusInternalServerError)
				return
			}

			fmt.Println(row)
			// TODO print row to w (as part of a json [{"in":csvLine, "out":[[],[],...,[]]}])
		}
		err = rows.Err()
		if err != nil {
			http.Error(w,
				fmt.Sprintf("Error executing query: %v\n\n%s", err, helpMessege), http.StatusInternalServerError)
			return
		}
	}
}
