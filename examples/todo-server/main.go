package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jakubtomas-cz/mongopher"
)

var col mongopher.Collection

func main() {
	ctx := context.Background()

	client, err := mongopher.Connect(ctx, "mongodb://localhost:27017", "todo_demo")
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(ctx)

	col = client.Collection("todos")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /todos", listTodos)
	mux.HandleFunc("POST /todos", createTodo)
	mux.HandleFunc("GET /todos/{id}", getTodo)
	mux.HandleFunc("PATCH /todos/{id}", updateTodo)
	mux.HandleFunc("DELETE /todos/{id}", deleteTodo)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// listTodos returns all todos. Pass ?done=true or ?done=false to filter by status.
//
//	curl http://localhost:8080/todos
//	curl "http://localhost:8080/todos?done=false"
//	curl "http://localhost:8080/todos?done=true"
func listTodos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var filter mongopher.Filter
	switch r.URL.Query().Get("done") {
	case "true":
		filter = mongopher.Eq("done", true)
	case "false":
		filter = mongopher.Eq("done", false)
	default:
		filter = mongopher.EmptyFilter()
	}

	docs, err := col.Find(ctx, filter, mongopher.WithSort("createdAt", mongopher.ASC))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(docs)
}

// createTodo creates a new todo. Body: {"title": "..."}
//
//	curl -X POST http://localhost:8080/todos \
//	     -H "Content-Type: application/json" \
//	     -d '{"title":"Buy milk"}'
func createTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	title, ok := input["title"].(string)
	if !ok || title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}

	input["done"] = false
	input["createdAt"] = time.Now().UTC()

	doc, err := mongopher.Marshal(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	result, err := col.InsertOne(ctx, doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	input["_id"] = result.InsertedID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(input)
}

// getTodo returns a single todo by ID.
//
//	curl http://localhost:8080/todos/<id>
func getTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	filter, err := mongopher.FilterByID(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	doc, err := col.FindOne(ctx, filter)
	if errors.Is(err, mongopher.ErrNoDocuments) {
		writeError(w, http.StatusNotFound, errors.New("todo not found"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(doc)
}

// updateTodo patches a todo. Body: {"title": "...", "done": true}
//
//	curl -X PATCH http://localhost:8080/todos/<id> \
//	     -H "Content-Type: application/json" \
//	     -d '{"done":true}'
func updateTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	filter, err := mongopher.FilterByID(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(patch) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("nothing to update"))
		return
	}

	patchJSON, err := mongopher.Marshal(patch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	doc, err := col.FindOneAndUpdate(ctx, filter, mongopher.Set(patchJSON), mongopher.WithReturnAfter())
	if errors.Is(err, mongopher.ErrNoDocuments) {
		writeError(w, http.StatusNotFound, errors.New("todo not found"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(doc)
}

// deleteTodo removes a todo by ID.
//
//	curl -X DELETE http://localhost:8080/todos/<id>
func deleteTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	filter, err := mongopher.FilterByID(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	result, err := col.DeleteOne(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if result.DeletedCount == 0 {
		writeError(w, http.StatusNotFound, errors.New("todo not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
