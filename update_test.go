package mongopher_test

import (
	"testing"

	"github.com/jakubtomas-cz/mongopher"
)

func TestSet(t *testing.T) {
	got := mongopher.Set([]byte(`{"age":31}`))
	want := `{"$set":{"age":31}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestUnset(t *testing.T) {
	got := mongopher.Unset([]byte(`{"deletedAt":""}`))
	want := `{"$unset":{"deletedAt":""}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestInc(t *testing.T) {
	got := mongopher.Inc([]byte(`{"loginCount":1}`))
	want := `{"$inc":{"loginCount":1}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestPush(t *testing.T) {
	got := mongopher.Push([]byte(`{"tags":"go"}`))
	want := `{"$push":{"tags":"go"}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestPull(t *testing.T) {
	got := mongopher.Pull([]byte(`{"tags":"deprecated"}`))
	want := `{"$pull":{"tags":"deprecated"}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestAddToSet(t *testing.T) {
	got := mongopher.AddToSet([]byte(`{"roles":"editor"}`))
	want := `{"$addToSet":{"roles":"editor"}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestRename(t *testing.T) {
	got := mongopher.Rename([]byte(`{"oldField":"newField"}`))
	want := `{"$rename":{"oldField":"newField"}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
