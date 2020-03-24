package errors

import (
	"github.com/neuronlabs/jsonapi"
)

// Creator is the function used to create new Error instance.
type Creator func() *jsonapi.Error
