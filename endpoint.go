package handler

import (
	"net/http"

	"github.com/neuronlabs/neuron-core/mapping"
)

// EndpointHandler is the structure that allows to customize predefined handler func.
type EndpointHandler struct {
	handler  func(*mapping.ModelStruct, string) http.HandlerFunc
	basePath string
	model    *mapping.ModelStruct
}

// BasePath sets the BasePath for given endpoint handler.
func (e *EndpointHandler) BasePath(basePath string) *EndpointHandler {
	e.basePath = basePath
	return e
}

// Handler returns preset handler function.
func (e *EndpointHandler) Handler() http.HandlerFunc {
	return e.handler(e.model, e.basePath)
}
