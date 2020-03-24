package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	"github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

func (h *Creator) PatchWith(model interface{}) *EndpointHandler {
	return &EndpointHandler{
		handler: h.handlePatch,
		model:   h.c.MustGetModelStruct(model),
	}
}

func (h *Creator) Patch(model interface{}) http.HandlerFunc {
	return h.handlePatch(h.c.MustGetModelStruct(model), "")
}

func (h *Creator) handlePatch(model *mapping.ModelStruct, basePath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var buf *bytes.Buffer
		reader := io.Reader(req.Body)
		// for debug purpose prepare the tee reader
		if log.Level().IsAllowed(log.LDEBUG3) {
			buf = &bytes.Buffer{}
			reader = io.TeeReader(req.Body, buf)
		}

		id := CtxMustGetID(req.Context())
		if id == "" {
			log.Debugf("[PATCH][%s] Empty id params", model.Collection())
			err := errors.ErrBadRequest()
			err.Detail = "Provided empty 'id' in url"
			h.marshalErrors(rw, req, 0, err)
			return
		}

		idURLValue, err := model.Primary().ValueFromString(id)
		if err != nil {
			err := errors.ErrInvalidURI()
			err.Detail = "Provided invalid 'id' value in url"
			log.Debug2f("[PATCH][%s] invalid 'id' value: '%v'", model.Collection(), id)
			h.marshalErrors(rw, req, 0, err)
			return
		}

		s, err := jsonapi.UnmarshalSingleScopeC(h.c, reader, model, h.jsonapiUnmarshalOptions())
		if err != nil {
			if log.Level().IsAllowed(log.LDEBUG3) {
				log.Debug3f("Unmarshal value: '%s' failed: %v", buf.String(), err)
			}
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		idDataValue, err := h.getFieldValue(s.Value, s.Struct().Primary())
		if err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		if idDataValue != idURLValue {
			err := errors.ErrIDConflict()
			err.Detail = fmt.Sprintf("URL id value: '%s' doesn't match input data id value: '%v'", id, idDataValue)
			log.Debug2f("[PATCH][%s] %s", model.Collection(), err.Detail)
			h.marshalErrors(rw, req, 0, err)
			return
		}

		ctx := req.Context()

		// execute the before patcher API hook if given model defines it.
		if beforePatchHook, ok := Hooks.getHook(model, BeforePatch); ok {
			if err = beforePatchHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		if err = s.PatchContext(ctx); err != nil {
			log.Debug2f("[PATCH][%s][%s] failed: %v ", model.Collection(), s.ID(), err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		// execute the before patcher API hook if given model defines it.
		if afterPatchHook, ok := Hooks.getHook(model, AfterPatch); ok {
			if err = afterPatchHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		if req.Header.Get("Accept") != jsonapi.MediaType {
			log.Debug3f("[PATCH][%s][%s] No 'Accept' Header - returning HTTP Status: No Content - 204", model.Collection(), s.ID())
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		getScope := query.NewModelC(h.c, model, false)
		if err = getScope.FilterField(query.NewFilter(model.Primary(), query.OpEqual, idDataValue)); err != nil {
			log.Errorf("[PATCH][SCOPE][%s] Adding param primary filter to return content scope failed: %v", err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		if err = getScope.GetContext(ctx); err != nil {
			log.Debugf("[PATCH][%s][%s] Getting resource after patching failed: %v", model.Collection(), s.ID(), err)
			rw.WriteHeader(http.StatusNoContent)
			return
		}
		if basePath == "" {
			basePath = h.basePath()
		}

		options := &jsonapi.MarshalOptions{Link: jsonapi.LinkOptions{
			Type:       jsonapi.ResourceLink,
			BaseURL:    basePath,
			RootID:     id,
			Collection: model.Collection(),
		}}
		h.marshalScope(getScope, rw, req, http.StatusOK, options)
	}
}
