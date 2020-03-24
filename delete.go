package handler

import (
	"net/http"

	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	"github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// Delete is the JSONAPI DELETE http handler for provided 'model'.
func (h *Creator) Delete(model interface{}) http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	return h.handleDelete(mappedModel)
}

func (h *Creator) handleDelete(model *mapping.ModelStruct) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		id := CtxMustGetID(ctx)
		if id == "" {
			// if the function would not contain 'id' parameter.
			log.Debugf("[DELETE] Empty id params: %v", id)
			jsonapiError := errors.ErrInvalidQueryParameter()
			jsonapiError.Detail = "Provided empty id in the query URL"
			h.marshalErrors(rw, req, 0, jsonapiError)
			return
		}
		idValue, err := model.Primary().ValueFromString(id)
		if err != nil {
			log.Debugf("[DELETE][%s] Invalid URL id value: '%s': '%v'", model.Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		s := query.NewModelC(h.c, model, false)
		if err = s.FilterField(query.NewFilter(model.Primary(), query.OpEqual, idValue)); err != nil {
			// this should not occur - primary field's model must match scope's model.
			log.Errorf("[DELETE][%s] Adding param primary filter with value: '%s' failed: %v", model.Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		model := s.Struct()
		// execute the before deleter API hook if given model defines it.
		if beforeDeleteHook, ok := Hooks.getHook(model, BeforeDelete); ok {
			if err = beforeDeleteHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		if err = s.DeleteContext(ctx); err != nil {
			log.Debugf("[DELETE][SCOPE][%s] Delete /%s/%s root scope failed: %v", s.Struct().Collection(), s.Struct().Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		// execute the after deleter API hook if given model defines it.
		if afterDeleteHook, ok := Hooks.getHook(model, AfterDelete); ok {
			if err = afterDeleteHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}
		rw.WriteHeader(http.StatusNoContent)
	}
}
