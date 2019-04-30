package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	json "github.com/json-iterator/go"
	_ "github.com/mattn/go-sqlite3"
)

const version = "1.3.2"

func main() {
	err := cmd(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

func cmd(cmdArgs []string) error {
	// Parse cmd args
	var flagSet = flag.NewFlagSet("cmd flags", flag.ContinueOnError)

	var dbPath string
	var queryString string
	var serverPort uint

	flagSet.StringVar(&dbPath, "db", "", "Filesystem path of the SQLite database")
	flagSet.StringVar(&queryString, "query", "", "SQL query to prepare for")
	flagSet.UintVar(&serverPort, "port", 80, "HTTP port to listen on")

	err := flagSet.Parse(cmdArgs)
	if err != nil {
		return err
	}

	// Init db and query
	queryHandler, err := initQueryHandler(dbPath, queryString, serverPort)
	if err != nil {
		return err
	}

	// Start the server
	log.Printf("Starting server on port %d...\n", serverPort)
	log.Printf("Starting server with query '%s'...\n", queryString)

	http.HandleFunc("/query", queryHandler)
	err = http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil)

	return err
}

type queryResult struct {
	In      []string        `json:"in"`
	Headers []string        `json:"headers"`
	Out     [][]interface{} `json:"out"`
}

func initQueryHandler(dbPath string, queryString string, serverPort uint) (func(w http.ResponseWriter, r *http.Request), error) {
	// Init db and query

	if dbPath == "" {
		return nil, fmt.Errorf("Must provide --db param")
	}
	if queryString == "" {
		return nil, fmt.Errorf("Must provide --query param")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Database file '%s' doesn't exist", dbPath)
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rw&cache=shared&_journal_mode=WAL", dbPath))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	queryStmt, err := db.Prepare(queryString)
	if err != nil {
		db.Close()
		return nil, err
	}

	helpMessage := buildHelpMessage("", queryString, queryStmt, serverPort)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "SQLiteQueryServer v"+version)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if r.URL.Path != "/query" {
			http.Error(w, helpMessage, http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			http.Error(w, helpMessage, http.StatusMethodNotAllowed)
			return
		}

		// Init fullResponse
		fullResponse := []queryResult{}

		reqCsvReader := csv.NewReader(r.Body)
		reqCsvReader.FieldsPerRecord = -1

		// Iterate over each query
		for {
			csvRecord, err := reqCsvReader.Read()
			if err == io.EOF || err == http.ErrBodyReadAfterClose {
				// EOF || last line is without \n
				break
			} else if err != nil {
				http.Error(w, fmt.Sprintf("\n\nError reading request body: %v\n\n%s", err, helpMessage), http.StatusInternalServerError)
				return
			}

			// Init queryResponse
			// Set queryResponse.Headers to the query's params (the fields of the csv record)
			var queryResponse queryResult
			queryResponse.In = csvRecord

			queryParams := make([]interface{}, len(csvRecord))
			for i := range csvRecord {
				queryParams[i] = csvRecord[i]
			}

			rows, err := queryStmt.Query(queryParams...)
			if err != nil {
				http.Error(w, fmt.Sprintf("\n\nError executing query for params %#v: %v\n\n%s", csvRecord, err, helpMessage), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			// Set queryResponse.Headers to the query's columns
			// Init queryResponse.Out
			cols, err := rows.Columns()
			if err != nil {
				http.Error(w, fmt.Sprintf("\n\nError reading columns for query with params %#v: %v\n\n%s", csvRecord, err, helpMessage), http.StatusInternalServerError)
				return
			}

			queryResponse.Headers = cols
			queryResponse.Out = make([][]interface{}, 0)

			// Iterate over returned rows for this query
			// Append each row to queryResponse.Out
			for rows.Next() {
				row := make([]interface{}, len(cols))
				pointers := make([]interface{}, len(row))

				for i := range row {
					pointers[i] = &row[i]
				}

				err = rows.Scan(pointers...)
				if err != nil {
					http.Error(w, fmt.Sprintf("\n\nError reading query results for params %#v: %v\n\n%s", csvRecord, err, helpMessage), http.StatusInternalServerError)
					return
				}

				queryResponse.Out = append(queryResponse.Out, row)
			}
			err = rows.Err()
			if err != nil {
				http.Error(w, fmt.Sprintf("\n\nError executing query: %v\n\n%s", err, helpMessage), http.StatusInternalServerError)
				return
			}

			fullResponse = append(fullResponse, queryResponse)
		}

		// Return json
		w.Header().Add("Content-Type", "application/json")

		answerJSON, err := json.Marshal(fullResponse)
		if err != nil {
			http.Error(w, fmt.Sprintf("\n\nError encoding json: %v\n\n%s", err, helpMessage), http.StatusInternalServerError)
			return
		}

		_, err = w.Write(answerJSON)
		if err != nil {
			http.Error(w, fmt.Sprintf("\n\nError sending json to client: %v\n\n%s", err, helpMessage), http.StatusInternalServerError)
			return
		}
	}, nil
}

func buildHelpMessage(helpMessage string, queryString string, queryStmt *sql.Stmt, serverPort uint) string {
	helpMessage += fmt.Sprintf(`Query:
	%s

`, queryString)

	queryParamsCount := countParams(queryStmt, queryString)
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

	return helpMessage
}

func countParams(queryStmt *sql.Stmt, queryString string) int {
	// Query with 0 params
	rows, err := queryStmt.Query()
	if err == nil {
		// Query went fine, this means it has 0 params
		rows.Close()
		return 0
	}

	// Query returned an error
	// Parse the error to get the expected params count
	regex := regexp.MustCompile(`sql: expected (\p{N}) arguments, got 0`)
	regexSubmatches := regex.FindAllStringSubmatch(err.Error(), 1)
	if len(regexSubmatches) != 1 || len(regexSubmatches[0]) != 2 {
		// This is weird
		// queryStmt is prepared (compiled) so it is valid
		// but yet there was an error executing queryStmt
		// Return best guess
		// TODO: Should we maybe return an error and kill the server?
		return strings.Count(queryString, "?")
	}
	count, err := strconv.Atoi(regexSubmatches[0][1])
	if err != nil {
		// This is even weirder
		// The regex is \p{N}+ (unicode number sequence) and there was a match,
		// but converting it from string to int returned an error
		// Return best guess
		// TODO: Should we maybe return an error and kill the server?
		return strings.Count(queryString, "?")
	}
	return count
}
