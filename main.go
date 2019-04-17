package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
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
	outpoutEncoder := json.NewEncoder(w)
	fmt.Fprintf(w, "[")

	reqCsvReader := csv.NewReader(r.Body)
	reqCsvReader.ReuseRecord = true

	isFirstQuery := true
	for {
		csvLine, err := reqCsvReader.Read()
		if err == io.EOF || err == http.ErrBodyReadAfterClose /*last line without \n*/ {
			break
		} else if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError reading request body: %v\n\n%s", err, helpMessege), http.StatusInternalServerError)
			return
		}

		if !isFirstQuery {
			fmt.Fprintf(w, ",")
		}
		isFirstQuery = false

		queryParams := make([]interface{}, len(csvLine))
		for i := range csvLine {
			queryParams[i] = csvLine[i]
		}

		rows, err := queryStmt.Query(queryParams...)
		if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError executing query for params %v: %v\n\n%s", csvLine, err, helpMessege), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError executing query for params %v: %v\n\n%s", csvLine, err, helpMessege), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, `{"in":`)
		outpoutEncoder.Encode(csvLine)
		fmt.Fprintf(w, ",")
		fmt.Fprintf(w, `"headers":`)
		outpoutEncoder.Encode(cols)
		fmt.Fprintf(w, `,"out":[`)

		isFirstRow := true
		for rows.Next() {
			if !isFirstRow {
				fmt.Fprintf(w, ",")
			}
			isFirstRow = false

			row := make([]interface{}, len(cols))
			pointers := make([]interface{}, len(row))

			for i := range row {
				pointers[i] = &row[i]
			}

			err = rows.Scan(pointers...)
			if err != nil {
				http.Error(w,
					fmt.Sprintf("\n\nError reading query results for params %v: %v\n\n%s", csvLine, err, helpMessege), http.StatusInternalServerError)
				return
			}

			outpoutEncoder.Encode(row)
		}
		err = rows.Err()
		if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError executing query: %v\n\n%s", err, helpMessege), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "]}")
	}
	fmt.Fprintf(w, "]")
}
