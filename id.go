package handler

import (
	"context"
)

type idKeyStruct struct{}

// IDKey is the context key that should match the value of 'ID' for the handlers requests.
var IDKey = &idKeyStruct{}

// CtxMustGetID gets ID from the context or panics if no id is found there.
// ID should be in the raw string form.
func CtxMustGetID(ctx context.Context) string {
	id, ok := ctx.Value(IDKey).(string)
	if !ok {
		panic("no 'id' stored in the context")
	}
	return id
}

// CtxSetID sets the 'id' value in the given 'ctx' and returns it.
func CtxSetID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, IDKey, id)
}
