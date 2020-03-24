package handler

import (
	"net/http"
	"strings"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/annotation"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	handlerErrors "github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/errors/class"
	"github.com/neuronlabs/jsonapi-handler/log"
)

type EndpointHandler struct {
	handler  func(*mapping.ModelStruct, string) http.HandlerFunc
	basePath string
	model    *mapping.ModelStruct
}

func (e *EndpointHandler) BasePath(basePath string) *EndpointHandler {
	e.basePath = basePath
	return e
}

func (e *EndpointHandler) Handler() http.HandlerFunc {
	return e.handler(e.model, e.basePath)
}

func (h *Creator) GetWith(model interface{}) *EndpointHandler {
	return &EndpointHandler{
		model:   h.c.MustGetModelStruct(model),
		handler: h.handleGet,
	}
}

func (h *Creator) Get(model interface{}) http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	return h.handleGet(mappedModel, "")
}

func (h *Creator) handleGet(model *mapping.ModelStruct, basePath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		s, err := h.createGetScope(req, model)
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}
		log.Debug3f("Fieldset: %v", s.Fieldset)

		// execute the before patcher API hook if given model defines it.
		if beforeGetHook, ok := Hooks.getHook(model, BeforeGet); ok {
			if err = beforeGetHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
				return
			}
		}

		if err := s.GetContext(ctx); err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}

		// execute the before patcher API hook if given model defines it.
		if afterGetHook, ok := Hooks.getHook(model, AfterGet); ok {
			if err = afterGetHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
				return
			}
		}

		if basePath != "" {
			basePath = h.basePath()
		}

		options := &jsonapi.MarshalOptions{Link: jsonapi.LinkOptions{
			Type:       jsonapi.ResourceLink,
			BaseURL:    basePath,
			RootID:     CtxMustGetID(ctx),
			Collection: model.Collection(),
		}}
		h.marshalScope(s, rw, req, http.StatusOK, options)
	}
}

func (h *Creator) createGetScope(req *http.Request, model *mapping.ModelStruct) (*query.Scope, error) {
	id := CtxMustGetID(req.Context())
	if id == "" {
		log.Errorf("ID value stored in the context is empty.")
		err := errors.NewDet(class.QueryInvalidURL, "invalid 'id' url parameter")
		err.SetDetails("Provided empty ID in query url")
		return nil, err
	}
	// get the value from the string
	idValue, err := model.Primary().ValueFromString(id)
	if err != nil {
		log.Debugf("[GET][%s] Invalid URL id value: '%s': '%v'", model.Collection(), id, err)
		return nil, err
	}

	var multiErrors errors.MultiError

	s := query.NewModelC(h.c, model, false)
	q := req.URL.Query()

	included, ok := q[query.ParamInclude]
	if ok {
		var splitIncludes []string
		for _, includedQ := range included {
			splitIncludes = append(splitIncludes, strings.Split(includedQ, annotation.Separator)...)
		}
		log.Debug2f("Including fields: %v", splitIncludes)
		err := s.IncludeFields(splitIncludes...)
		if err != nil {
			return nil, err
		}
	}

	languages, ok := q[query.ParamLanguage]
	if ok {
		err := h.queryParameterLanguage(s, languages[0])
		if err != nil {
			return nil, err
		}
	}

	if err = s.FilterField(query.NewFilter(model.Primary(), query.OpEqual, idValue)); err != nil {
		log.Errorf("Creating preset primary filter in GET request for model: '%s' failed: %v.", model.Collection(), err)
		return nil, err
	}

	for key, values := range q {
		if len(multiErrors) >= h.QueryErrorsLimit {
			return nil, multiErrors
		}

		if len(values) > 1 {
			err := errors.NewDetf(class.QueryInvalidParameter, "provided invalid query parameters")
			err.SetDetailsf("The query parameter: '%s' used more than once.", key)
			multiErrors = append(multiErrors, err)
			continue
		}

		value := values[0]
		var err error
		switch {
		case key == query.ParamInclude, key == query.ParamLanguage:
			continue
		case strings.HasPrefix(key, query.ParamFields):
			err = h.queryParameterFields(s, key, value)
		// case strings.HasPrefix(key, query.ParamFilter):
		// 	err = h.queryParameterFilters(s, key, value)
		case key == QueryParamLinks:
			err = h.queryParameterLinks(s, key, value)
		default:
			err = h.defaultQueryParameter(key)
		}

		if err != nil {
			if ce, ok := err.(errors.ClassError); ok {
				multiErrors = append(multiErrors, ce)
			} else {
				return nil, err
			}
		}
	}

	if len(multiErrors) > 0 {
		return s, multiErrors
	}
	return s, nil
}
