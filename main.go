package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Counter int    `json:"counter"`
}

type application struct {
	db *sql.DB
}

func NewApplication() (*application, error) {
	db, err := sql.Open("sqlite3", "database.sqlite")
	if err != nil {
		return nil, err
	}

	return &application{db: db}, nil
}

func (app *application) Close() {
	app.db.Close()
}

func (app *application) CreateSchema() {
	f := func(q string, args ...interface{}) {
		_, err := app.db.Exec(q, args...)
		if err != nil {
			log.Fatalf("Unable to execute database statement: %v", err)
		}
	}

	f("CREATE TABLE IF NOT EXISTS user (" +
		"id INTEGER PRIMARY KEY, " +
		"name TEXT NOT NULL)")

	f("CREATE TABLE IF NOT EXISTS counter (" +
		"id INTEGER PRIMARY KEY, " +
		"user_id INTEGER REFERENCES user (id), " +
		"timestamp DATETIME DEFAULT CURRENT_TIMESTAMP)")
}

func (app *application) ListenAndServe(bind string) {
	http.Handle("/", http.FileServer(http.Dir("www")))

	http.HandleFunc("/api/increment", func(w http.ResponseWriter, r *http.Request) {
		app.handleIncrement(w, r)
	})

	http.HandleFunc("/api/last", func(w http.ResponseWriter, r *http.Request) {
		app.handleLast(w, r)
	})

	http.HandleFunc("/api/total", func(w http.ResponseWriter, r *http.Request) {
		app.handleTotal(w, r)
	})

	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		app.handleUser(w, r)
	})

	log.Printf("Listening on %v", bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}

func (app *application) getUsers() []User {
	users := make([]User, 0)

	rows, err := app.db.Query(
		"SELECT user.id, user.name, COUNT(counter.timestamp) " +
			"FROM user LEFT OUTER JOIN counter ON counter.user_id = user.id " +
			"GROUP BY user.id " +
			"ORDER BY user.name ASC")
	if err != nil {
		log.Fatalf("Unable to query database: %v", err)
	}

	for rows.Next() {
		var u User
		err := rows.Scan(&u.Id, &u.Name, &u.Counter)
		if err != nil {
			log.Fatal(err)
		}
		users = append(users, u)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	return users
}

func (app *application) getTotal() (total int, this_week int, today int) {
	f := func(p *int, q string) {
		row := app.db.QueryRow(q)
		err := row.Scan(p)
		if err != nil {
			log.Fatalf("Unable to execute query: %v", err)
		}
	}
	f(&total, "SELECT COUNT(*) FROM counter")
	f(&this_week, "SELECT COUNT(*) FROM counter WHERE timestamp > DATETIME('now', 'start of day')")
	f(&today, "SELECT COUNT(*) FROM counter WHERE timestamp > DATETIME('now', 'weekday 0', '-7 days')")
	return
}

func (app *application) getMinutesSinceLast() (elapsed float64) {
	rows, err := app.db.Query(
		"SELECT 24*60 * (julianday(CURRENT_TIMESTAMP) - COALESCE(MAX(julianday(timestamp)), 0)) " +
			"FROM counter")
	if err != nil {
		goto fail
	}

	elapsed = math.Inf(1)
	if rows.Next() {
		err = rows.Scan(&elapsed)
		if err != nil {
			goto fail
		}
	}
	rows.Close()
	return

fail:
	log.Fatalf("Unable to execute query: %v", err)
	return
}

func (app *application) handleUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	log.Printf("request for user list...\n")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(app.getUsers())
}

func (app *application) handleTotal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	log.Printf("request for total counters...\n")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	total, this_week, today := app.getTotal()
	json.NewEncoder(w).Encode(map[string]int{
		"total":     total,
		"this_week": this_week,
		"today":     today,
	})
}

func (app *application) handleIncrement(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	entries := r.Form["user"]
	if len(entries) != 1 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, err := strconv.Atoi(entries[0])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	elapsed := app.getMinutesSinceLast()
	if elapsed < 45 {
		log.Printf("Too many requests for user id %v\n", user)
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	log.Printf("incrementing counter for user %v...\n", user)
	_, err = app.db.Exec("INSERT INTO counter (user_id) VALUES (?)", user)
	if err != nil {
		log.Fatalf("Unable to update counter: %v", err)
	}
}

func (app *application) handleLast(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	log.Printf("request for last...\n")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	elapsed := app.getMinutesSinceLast()
	if math.IsInf(elapsed, 0) {
		elapsed = -1
	}

	json.NewEncoder(w).Encode(map[string]float64{
		"elapsed": elapsed,
	})
}

func main() {
	app, err := NewApplication()
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	defer app.Close()
	app.CreateSchema()
	app.ListenAndServe(":8080")
}
