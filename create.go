package handler

import (
	"net/http"

	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/mapping"

	"github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// CreateWith returns JSONAPI create method endpoint handler.
func (h *Creator) CreateWith(model interface{}) *EndpointHandler {
	return &EndpointHandler{
		handler: h.handleCreate,
		model:   h.c.MustGetModelStruct(model),
	}
}

// Create returns JSONAPI create method handler function for the provided 'model'.
func (h *Creator) Create(model interface{}) http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	return h.handleCreate(mappedModel, "")
}

func (h *Creator) handleCreate(model *mapping.ModelStruct, basePath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// unmarshal the input from the request body.
		s, err := jsonapi.UnmarshalSingleScopeC(h.c, req.Body, model, h.jsonapiUnmarshalOptions())
		if err != nil {
			log.Debugf("Unmarshal scope for: '%s' failed: %v", model.Collection(), err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		// check if the primary key is unmarshaled.
		_, isPrimary := s.Fieldset[model.Primary().NeuronName()]
		if isPrimary {
			// if the model doesn't allow custom client id, then return an error.
			if !model.AllowClientID() {
				log.Debug2f("Creating: '%s' with client-generated ID is forbidden", s.Struct().Collection())
				err := errors.ErrInvalidJSONFieldValue()
				err.Detail = "Client-Generated ID is not allowed for this model."
				err.Status = "403"
				h.marshalErrors(rw, req, http.StatusForbidden, err)
				return
			}
		}
		// get the context.
		ctx := req.Context()

		if beforeCreateHook, ok := Hooks.getHook(s.Struct(), BeforeCreate); ok {
			if err = beforeCreateHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		// execute the create query.
		if err = s.CreateContext(req.Context()); err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		// execute the after creator API hook if given model defines it.
		if afterCreateHook, ok := Hooks.getHook(s.Struct(), AfterCreate); ok {
			if err = afterCreateHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		// if the primary was provided in the input and if the config doesn't allow to return
		// created value with given client-id - return simple status NoContent
		if isPrimary && h.NoContentOnCreate {
			// if the primary was provided
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		// get the primary field value so that it could be used for the jsonapi marshal process.
		idDataValue, err := h.getFieldValue(s.Value, s.Struct().Primary())
		if err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}
		// get the string form value of the 'id'
		strValues := mapping.StringValues(idDataValue, nil)
		// by default marshal resource links
		linkType := jsonapi.ResourceLink
		// but if the config doesn't allow that - set 'jsonapi.NoLink'
		if !h.MarshalLinks {
			linkType = jsonapi.NoLink
		}
		// prepare the options to marshal jsonapi scope.
		options := &jsonapi.MarshalOptions{
			Link: jsonapi.LinkOptions{
				Type:       linkType,
				BaseURL:    h.getBasePath(basePath),
				Collection: s.Struct().Collection(),
				RootID:     strValues[0],
			},
		}
		h.marshalScope(s, rw, req, http.StatusCreated, options)
	}
}

func (h *Creator) getBasePath(basePath string) string {
	if basePath == "" {
		basePath = h.basePath()
	}
	return basePath
}
