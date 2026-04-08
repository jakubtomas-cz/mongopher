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

func TestMul(t *testing.T) {
	got := mongopher.Mul([]byte(`{"price":1.25}`))
	want := `{"$mul":{"price":1.25}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMin(t *testing.T) {
	got := mongopher.Min([]byte(`{"score":0}`))
	want := `{"$min":{"score":0}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMax(t *testing.T) {
	got := mongopher.Max([]byte(`{"score":100}`))
	want := `{"$max":{"score":100}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestPop(t *testing.T) {
	got := mongopher.Pop([]byte(`{"tags":1}`))
	want := `{"$pop":{"tags":1}}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
