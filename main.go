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

var queryParamsCount int

var helpMessage string

func init() {
	flag.StringVar(&dbPath, "db", "", "Filesystem path of the SQLite database")
	flag.StringVar(&queryString, "query", "", "SQL query to prepare for")
	flag.UintVar(&serverPort, "port", 80, "HTTP port to listen on")

	flag.Parse()

	if queryString == "" {
		log.Fatal("Must provide --query param")
	}
	if dbPath == "" {
		log.Fatal("Must provide --db param")
	}

	var err error
	db, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rw&cache=shared&_journal_mode=WAL", dbPath))
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(1)

	queryStmt, err = db.Prepare(queryString)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Database:\n\t%s\n\n", dbPath)
	fmt.Printf("Port:\n\t%d\n\n", serverPort)

	buildHelpMessage()

	fmt.Printf(helpMessage)
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
		http.Error(w, helpMessage, http.StatusNotFound)
		return
	}
	if r.Method != "POST" {
		http.Error(w, helpMessage, http.StatusMethodNotAllowed)
		return
	}

	wFlusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w,
			fmt.Sprintf("Error creating a stream writer.\n\n%s", helpMessage), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	outpoutEncoder := json.NewEncoder(w)
	// start printing the outer array
	fmt.Fprintf(w, "[")
	wFlusher.Flush()

	reqCsvReader := csv.NewReader(r.Body)
	reqCsvReader.ReuseRecord = true

	isFirstQuery := true
	for {
		csvLine, err := reqCsvReader.Read()
		if err == io.EOF || err == http.ErrBodyReadAfterClose /* last line is without \n */ {
			break
		} else if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError reading request body: %v\n\n%s", err, helpMessage), http.StatusInternalServerError)
			return
		}

		if !isFirstQuery {
			// print comma between queries results
			fmt.Fprintf(w, ",")
			wFlusher.Flush()
		}
		isFirstQuery = false

		queryParams := make([]interface{}, len(csvLine))
		for i := range csvLine {
			queryParams[i] = csvLine[i]
		}

		rows, err := queryStmt.Query(queryParams...)
		if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError executing query for params %#v: %v\n\n%s", csvLine, err, helpMessage), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError executing query for params %#v: %v\n\n%s", csvLine, err, helpMessage), http.StatusInternalServerError)
			return
		}

		// start printing a query result
		fmt.Fprintf(w, `{"in":`)
		outpoutEncoder.Encode(csvLine)
		fmt.Fprintf(w, ",")
		fmt.Fprintf(w, `"headers":`)
		outpoutEncoder.Encode(cols)
		fmt.Fprintf(w, `,"out":[`) // start printing the out rows array
		wFlusher.Flush()

		isFirstRow := true
		for rows.Next() {
			if !isFirstRow {
				// print comma between rows
				fmt.Fprintf(w, ",")
				wFlusher.Flush()
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
					fmt.Sprintf("\n\nError reading query results for params %#v: %v\n\n%s", csvLine, err, helpMessage), http.StatusInternalServerError)
				return
			}

			// print a result row
			outpoutEncoder.Encode(row)
		}
		err = rows.Err()
		if err != nil {
			http.Error(w,
				fmt.Sprintf("\n\nError executing query: %v\n\n%s", err, helpMessage), http.StatusInternalServerError)
			return
		}

		// finish printing a query result
		fmt.Fprintf(w, "]}")
		wFlusher.Flush()
	}

	// finish printing the outer array
	fmt.Fprintf(w, "]\n")
	wFlusher.Flush()
}

func buildHelpMessage() {
	helpMessage += fmt.Sprintf(`Query:
	%s

`, queryString)

	queryParamsCount = strings.Count(queryString, "?")
	helpMessage += fmt.Sprintf(`Params count (question marks in query):
	%d

`, queryParamsCount)

	helpMessage += fmt.Sprintf(`Request examples:
	$ echo -e "$QUERY1_PARAM1,$QUERY1_PARAM2\n$QUERY2_PARAM1,$QUERY2_PARAM2" curl "http://$ADDRESS:%d/query" --data-binary @-
	$ curl "http://$ADDRESS:%d/query" -d "$PARAM_1,$PARAM_2,...,$PARAM_N"

	- Request must be a HTTP POST to "http://$ADDRESS:%d/query".
	- Request body must be a valid CSV.
	- Request body must not have a CSV header.
	- Each request body line is a different query.
	- Each param in a line corresponds to a query param (a question mark in the query string).

`, serverPort, serverPort, serverPort)

	helpMessage += fmt.Sprintf(`Response example:
	$ echo -e "github.com\none.one.one.one\ngoogle-public-dns-a.google.com" | curl "http://$ADDRESS:%d/query" --data-binary @-
	[
		{
			"in": ["github.com"],
			"headers": ["ip","dns"],
			"out": [
				["192.30.253.112","github.com"],
				["192.30.253.113","github.com"]
			]
		},
		{
			"in": ["one.one.one.one"],
			"headers": ["ip","dns"],
			"out": [
				["1.1.1.1","one.one.one.one"]
			]
		},
		{
			"in": ["google-public-dns-a.google.com"],
			"headers": ["ip","dns"],
			"out": [
				["8.8.8.8","google-public-dns-a.google.com"]
			]
		}
	]

	- Response is a JSON array (Content-Type: application/json).
	- Each element in the array:
		- Is a result of a query
		- Has an "in" fields which is an array of the input params (a request body line).
		- Has an "headers" fields which is an array of headers of the SQL query result.
		- Has an "out" field which is an array of arrays of results. Each inner array is a result row.
	- Element #1 is the result of query #1, Element #2 is the result of query #2, and so forth.
`, serverPort)
}
