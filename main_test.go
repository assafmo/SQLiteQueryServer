package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	json "github.com/json-iterator/go"
)

var dbPath = "./db_example/ip_dns.db"

func TestResultCount(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})

	reqString := "github.com\none.one.one.one\ngoogle-public-dns-a.google.com"

	req := httptest.NewRequest("POST",
		"http://example.org/query",
		strings.NewReader(reqString))
	w := httptest.NewRecorder()
	queryHandler, err := initQueryHandler(dbPath, "SELECT * FROM ip_dns WHERE dns = ?", 0)
	if err != nil {
		t.Fatal(err)
	}
	queryHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf(`resp.StatusCode (%d) != http.StatusOK (%d)`, resp.StatusCode, http.StatusOK)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf(`resp.Header.Get("Content-Type") (%s) != "application/json"`, resp.Header.Get("Content-Type"))
	}

	var resultsFromServer []interface{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&resultsFromServer)
	if err != nil {
		t.Fatal(err)
	}

	if len(resultsFromServer) != 3 {
		t.Fatal(`len(resultsFromServer) != 3`)
	}
}

func TestAnswersOrder(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})

	reqString := "github.com\none.one.one.one\ngoogle-public-dns-a.google.com"

	req := httptest.NewRequest("POST",
		"http://example.org/query",
		strings.NewReader(reqString))
	w := httptest.NewRecorder()
	queryHandler, err := initQueryHandler(dbPath, "SELECT * FROM ip_dns WHERE dns = ?", 0)
	if err != nil {
		t.Fatal(err)
	}
	queryHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf(`resp.StatusCode (%d) != http.StatusOK (%d)`, resp.StatusCode, http.StatusOK)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf(`resp.Header.Get("Content-Type") (%s) != "application/json"`, resp.Header.Get("Content-Type"))
	}

	var resultsFromServer []httpAnswer
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&resultsFromServer)
	if err != nil {
		t.Fatal(err)
	}

	if len(resultsFromServer) != 3 {
		t.Fatal(`len(resultsFromServer) != 3`)
	}

	if len(resultsFromServer[0].In) != 1 {
		t.Fatal(`len(resultsFromServer[0].In) != 1`)
	}
	if len(resultsFromServer[1].In) != 1 {
		t.Fatal(`len(resultsFromServer[1].In) != 1`)
	}
	if len(resultsFromServer[2].In) != 1 {
		t.Fatal(`len(resultsFromServer[2].In) != 1`)
	}

	if resultsFromServer[0].In[0] != "github.com" {
		t.Fatal(`resultsFromServer[0].In[0] != "github.com"`)
	}
	if resultsFromServer[1].In[0] != "one.one.one.one" {
		t.Fatal(`resultsFromServer[1].In[0] != "one.one.one.one"`)
	}
	if resultsFromServer[2].In[0] != "google-public-dns-a.google.com" {
		t.Fatal(`resultsFromServer[2].In[0] != "google-public-dns-a.google.com"`)
	}
}

func TestBadParamsCount(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})

	reqString := "github.com,1"

	req := httptest.NewRequest("POST",
		"http://example.org/query",
		strings.NewReader(reqString))
	w := httptest.NewRecorder()
	queryHandler, err := initQueryHandler(dbPath, "SELECT * FROM ip_dns WHERE dns = ?", 0)
	if err != nil {
		t.Fatal(err)
	}
	queryHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf(`resp.StatusCode (%d) != http.StatusInternalServerError (%d)`, resp.StatusCode, http.StatusInternalServerError)
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	respString := string(respBytes)

	if !strings.Contains(respString, "sql: expected 1 arguments, got 2") {
		t.Fatal(`Error string should contain "sql: expected 1 arguments, got 2"`)
	}
}

func TestBadPathRequest(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})

	reqString := `github.com`

	req := httptest.NewRequest("POST",
		"http://example.org/queri",
		strings.NewReader(reqString))
	w := httptest.NewRecorder()
	queryHandler, err := initQueryHandler(dbPath, "SELECT * FROM ip_dns WHERE dns = ?", 0)
	if err != nil {
		t.Fatal(err)
	}
	queryHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf(`resp.StatusCode (%d) != http.StatusNotFound (%d)`, resp.StatusCode, http.StatusNotFound)
	}
}

func TestBadMethodRequest(t *testing.T) {
	log.SetOutput(&bytes.Buffer{})

	req := httptest.NewRequest("GET",
		"http://example.org/query",
		nil)
	w := httptest.NewRecorder()
	queryHandler, err := initQueryHandler(dbPath, "SELECT * FROM ip_dns WHERE dns = ?", 0)
	if err != nil {
		t.Fatal(err)
	}
	queryHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf(`resp.StatusCode (%d) != http.StatusMethodNotAllowed (%d)`, resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
