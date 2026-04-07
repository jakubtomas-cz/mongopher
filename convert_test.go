package mongopher_test

import (
	"errors"
	"testing"

	"github.com/jakubtomas-cz/mongopher"
)

func TestMarshal(t *testing.T) {
	doc, err := mongopher.Marshal(map[string]any{"name": "Alice", "age": 30})
	if err != nil {
		t.Fatal(err)
	}
	m, err := mongopher.UnmarshalAs[map[string]any](doc)
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "Alice" || m["age"] != float64(30) {
		t.Fatalf("unexpected values: %v", m)
	}
}

func TestMarshal_Struct(t *testing.T) {
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	doc, err := mongopher.Marshal(user{Name: "Bob", Age: 25})
	if err != nil {
		t.Fatal(err)
	}
	u, err := mongopher.UnmarshalAs[user](doc)
	if err != nil {
		t.Fatal(err)
	}
	if u != (user{"Bob", 25}) {
		t.Fatalf("unexpected value: %v", u)
	}
}

func TestMarshal_InvalidValue(t *testing.T) {
	// channels cannot be marshalled to JSON
	_, err := mongopher.Marshal(make(chan int))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUnmarshalAs_Map(t *testing.T) {
	docs := []byte(`[{"name":"Alice","age":30},{"name":"Bob","age":25}]`)

	items, err := mongopher.UnmarshalAs[[]map[string]any](docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["name"] != "Alice" || items[0]["age"] != float64(30) {
		t.Fatalf("unexpected first item: %v", items[0])
	}
	if items[1]["name"] != "Bob" || items[1]["age"] != float64(25) {
		t.Fatalf("unexpected second item: %v", items[1])
	}
}

func TestUnmarshalAs_Array(t *testing.T) {
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	docs := []byte(`[{"name":"Alice","age":30},{"name":"Bob","age":25}]`)

	items, err := mongopher.UnmarshalAs[[]user](docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != (user{"Alice", 30}) {
		t.Fatalf("unexpected first item: %v", items[0])
	}
	if items[1] != (user{"Bob", 25}) {
		t.Fatalf("unexpected second item: %v", items[1])
	}
}

func TestUnmarshalAs_Single(t *testing.T) {
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	doc := []byte(`{"name":"Alice","age":30}`)

	u, err := mongopher.UnmarshalAs[user](doc)
	if err != nil {
		t.Fatal(err)
	}
	if u != (user{"Alice", 30}) {
		t.Fatalf("unexpected value: %v", u)
	}
}

func TestUnmarshalAs_Empty(t *testing.T) {
	type user struct{ Name string }

	items, err := mongopher.UnmarshalAs[[]user]([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestUnmarshalAs_InvalidJSON(t *testing.T) {
	type user struct{ Name string }

	_, err := mongopher.UnmarshalAs[user]([]byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}
