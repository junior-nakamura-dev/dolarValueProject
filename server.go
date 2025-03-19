package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"net/http"
	"time"
)

type RequestHistory struct {
	Bid string `json:"bid"`
}

type CurrencyData struct {
	Bid string `json:"bid"`
}

type ApiResponse struct {
	USDBRL CurrencyData `json:"USDBRL"`
}

func cotacaoHandler(w http.ResponseWriter, r *http.Request) {

	apiUrl := "https://economia.awesomeapi.com.br/json/last/USD-BRL"
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		http.Error(w, `{"error":"Error creating request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			http.Error(w, `{"error":"Request timed out"}`, http.StatusRequestTimeout)
			return
		}
		http.Error(w, `{"error":"Error fetching API"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf(`{"error":"Unexpected status code: %d"}`, resp.StatusCode), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, `{"error":"Error reading response body"}`, http.StatusInternalServerError)
		return
	}

	var result ApiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		http.Error(w, `{"error":"Error decoding JSON"}`, http.StatusInternalServerError)
		return
	}

	bid := result.USDBRL.Bid
	saveRequestHistory(RequestHistory{Bid: bid})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"bid":"%s"}`, bid)
}

func createDatabase() error {
	db, err := sql.Open("sqlite3", "request_history.db")
	if err != nil {
		return fmt.Errorf("error opening SQLite database: %v", err)
	}
	defer db.Close()
	// Create the table if it doesn't exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS request_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bid TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(createTableQuery); err != nil {
		return fmt.Errorf("error creating table: %v", err)
	}
	return nil
}

func saveRequestHistory(history RequestHistory) error {
	// Create a context with a timeout of 10 milliseconds
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Open a connection to the SQLite database
	db, err := sql.Open("sqlite3", "request_history.db")
	if err != nil {
		return fmt.Errorf("error opening SQLite database: %v", err)
	}
	defer db.Close()

	// Prepare the context-aware query execution
	insertQuery := `INSERT INTO request_history (bid) VALUES (?);`
	_, err = db.ExecContext(ctx, insertQuery, history.Bid)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("operation timed out")
		}
		return fmt.Errorf("error inserting into the database: %v", err)
	}

	return nil
}

func main() {
	errDb := createDatabase()
	if errDb != nil {
		panic(errDb)
	}
	http.HandleFunc("/cotacao", cotacaoHandler)

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		panic(err)
	}
}
