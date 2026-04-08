package mongopher

import "encoding/json"

// Set sets the value of specified fields. Fields not mentioned are left untouched.
//
//	col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
func Set(j []byte) []byte { return wrap("$set", j) }

// Unset removes the specified fields from the document entirely.
//
//	col.UpdateOne(ctx, filter, mongopher.Unset([]byte(`{"deletedAt":""}`)))
func Unset(j []byte) []byte { return wrap("$unset", j) }

// Inc increments the specified fields by the given amounts. Use a negative
// number to decrement.
//
//	col.UpdateOne(ctx, filter, mongopher.Inc([]byte(`{"loginCount":1}`)))
func Inc(j []byte) []byte { return wrap("$inc", j) }

// Mul multiplies the specified fields by the given factors.
//
//	col.UpdateOne(ctx, filter, mongopher.Mul([]byte(`{"price":1.25}`)))
func Mul(j []byte) []byte { return wrap("$mul", j) }

// Min updates a field only if the new value is less than the current value.
// Useful for tracking lowest-recorded values.
//
//	col.UpdateOne(ctx, filter, mongopher.Min([]byte(`{"score":0}`)))
func Min(j []byte) []byte { return wrap("$min", j) }

// Max updates a field only if the new value is greater than the current value.
// Useful for tracking highest-recorded values.
//
//	col.UpdateOne(ctx, filter, mongopher.Max([]byte(`{"score":100}`)))
func Max(j []byte) []byte { return wrap("$max", j) }

// Push appends a value to an array field. If the field does not exist it is
// created as a single-element array.
//
//	col.UpdateOne(ctx, filter, mongopher.Push([]byte(`{"tags":"go"}`)))
func Push(j []byte) []byte { return wrap("$push", j) }

// Pull removes all elements from an array field that match the given value or
// condition.
//
//	col.UpdateOne(ctx, filter, mongopher.Pull([]byte(`{"tags":"deprecated"}`)))
func Pull(j []byte) []byte { return wrap("$pull", j) }

// Pop removes an element from an array field. Use 1 to remove the last element,
// -1 to remove the first.
//
//	col.UpdateOne(ctx, filter, mongopher.Pop([]byte(`{"tags":1}`)))
func Pop(j []byte) []byte { return wrap("$pop", j) }

// AddToSet appends a value to an array field only if it is not already present,
// keeping the array free of duplicates.
//
//	col.UpdateOne(ctx, filter, mongopher.AddToSet([]byte(`{"roles":"editor"}`)))
func AddToSet(j []byte) []byte { return wrap("$addToSet", j) }

// Rename renames a field. The value is the new field name.
//
//	col.UpdateOne(ctx, filter, mongopher.Rename([]byte(`{"oldField":"newField"}`)))
func Rename(j []byte) []byte { return wrap("$rename", j) }

func wrap(op string, j []byte) []byte {
	out, _ := json.Marshal(map[string]json.RawMessage{op: j})
	return out
}
