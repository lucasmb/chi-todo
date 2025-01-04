package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
)

type Todo struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

var db *sql.DB
var tmpl *template.Template

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "todo.db") // SQLite database file
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
                CREATE TABLE IF NOT EXISTS todos (
                        id TEXT PRIMARY KEY,
                        text TEXT NOT NULL,
                        completed INTEGER NOT NULL DEFAULT 0,
                        created_at DATETIME NOT NULL
                )
        `)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to database")
}

type TemplateData struct {
	Title string
	Todos []Todo
}

// ... other code

func formatDate(t time.Time) string {
	return t.Format("2006-01-02 15:04:05") // Your desired format
}

func initTemplates() {
	var err error

	tmpl = template.New("main") // Create the main template set

	tmpl.Funcs(template.FuncMap{
		"formatDate": formatDate, // Register the function with the template
	})
	// Then parse the other templates
	tmpl, err = tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Parsing templates: %v", err) // More specific error message
	}

}

func getTodosHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, text, completed, created_at FROM todos ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "Failed to get todos", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var todo Todo
		var completedInt int
		err := rows.Scan(&todo.ID, &todo.Text, &completedInt, &todo.CreatedAt)
		if err != nil {
			http.Error(w, "Failed to get todos", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		todo.Completed = completedInt == 1 // Convert integer to boolean
		todos = append(todos, todo)
	}

	data := TemplateData{
		Title: "Todo List",
		Todos: todos,
	}

	var tmplToExec string
	if r.Header.Get("HX-Request") == "true" {
		tmplToExec = "_todo-list"
	} else {
		tmplToExec = "todos" // Execute the "todos" template, not "base" directly
	}
	err = tmpl.ExecuteTemplate(w, tmplToExec, data)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Println(err)
	}

}

func createTodoHandler(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "Text is required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String() // Use string representation for SQLite
	createdAt := time.Now()

	_, err := db.Exec("INSERT INTO todos (id, text, completed, created_at) VALUES (?, ?, ?, ?)", id, text, 0, createdAt)
	if err != nil {
		http.Error(w, "Failed to create todo", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	getTodosHandler(w, r)
}

func toggleTodoHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var completedInt int
	err = db.QueryRow("SELECT completed FROM todos WHERE id = ?", id).Scan(&completedInt)
	if err != nil {
		http.Error(w, "Could not find todo", http.StatusNotFound)
		return
	}

	completed := completedInt == 1
	newCompleted := !completed

	_, err = db.Exec("UPDATE todos SET completed = ? WHERE id = ?", newCompleted, id)
	if err != nil {
		http.Error(w, "Failed to update todo", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Retrieve the full todo from the database
	var updatedTodo Todo
	var createdAt time.Time
	err = db.QueryRow("SELECT id, text, completed, created_at FROM todos WHERE id = ?", id).Scan(&updatedTodo.ID, &updatedTodo.Text, &completedInt, &createdAt)
	if err != nil {
		http.Error(w, "Failed to retrieve todo", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	updatedTodo.Completed = completedInt == 1
	updatedTodo.CreatedAt = createdAt

	err = tmpl.ExecuteTemplate(w, "_todo-item", updatedTodo)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func main() {
	initDB()
	initTemplates()
	defer db.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", getTodosHandler)
	r.Post("/todos", createTodoHandler)
	r.Put("/todos/{id}", toggleTodoHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
