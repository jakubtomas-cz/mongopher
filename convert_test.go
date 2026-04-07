package mongopher_test

import (
	"errors"
	"testing"

	"github.com/jakubtomas-cz/mongopher"
)

func TestUnmarshal(t *testing.T) {
	docs := []byte(`[{"name":"Alice","age":30},{"name":"Bob","age":25}]`)

	items, err := mongopher.Unmarshal(docs)
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

func TestUnmarshal_Empty(t *testing.T) {
	items, err := mongopher.Unmarshal([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestUnmarshal_InvalidJSON(t *testing.T) {
	_, err := mongopher.Unmarshal([]byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestUnmarshalAs(t *testing.T) {
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	docs := []byte(`[{"name":"Alice","age":30},{"name":"Bob","age":25}]`)

	items, err := mongopher.UnmarshalAs[user](docs)
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

func TestUnmarshalAs_Empty(t *testing.T) {
	type user struct{ Name string }

	items, err := mongopher.UnmarshalAs[user]([]byte(`[]`))
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
