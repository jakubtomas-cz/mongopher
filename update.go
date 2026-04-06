package mongopher

// Set wraps a JSON object in a $set update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
func Set(j []byte) []byte { return wrap("$set", j) }

// Unset wraps a JSON object in a $unset update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.Unset([]byte(`{"deletedAt":""}`)))
func Unset(j []byte) []byte { return wrap("$unset", j) }

// Inc wraps a JSON object in a $inc update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.Inc([]byte(`{"loginCount":1}`)))
func Inc(j []byte) []byte { return wrap("$inc", j) }

// Push wraps a JSON object in a $push update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.Push([]byte(`{"tags":"go"}`)))
func Push(j []byte) []byte { return wrap("$push", j) }

// Pull wraps a JSON object in a $pull update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.Pull([]byte(`{"tags":"deprecated"}`)))
func Pull(j []byte) []byte { return wrap("$pull", j) }

// AddToSet wraps a JSON object in a $addToSet update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.AddToSet([]byte(`{"roles":"editor"}`)))
func AddToSet(j []byte) []byte { return wrap("$addToSet", j) }

// Rename wraps a JSON object in a $rename update operator.
//
//	col.UpdateOne(ctx, filter, mongopher.Rename([]byte(`{"oldField":"newField"}`)))
func Rename(j []byte) []byte { return wrap("$rename", j) }

func wrap(op string, j []byte) []byte {
	out := make([]byte, 0, len(op)+len(j)+6)
	out = append(out, `{"`...)
	out = append(out, op...)
	out = append(out, `":`...)
	out = append(out, j...)
	out = append(out, '}')
	return out
}
