package handler

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	neuronErrors "github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	"github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// GetRelated returns handler function for the
func (h *Creator) GetRelated(model interface{}, field string, basePath ...string) http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	sField, ok := mappedModel.RelationField(field)
	if !ok {
		log.Panicf("Field: '%s' not found for the model: '%s'", mappedModel.String())
	}
	var bp string
	if len(basePath) != 0 {
		bp = basePath[0]
	}
	return h.handleGetRelated(mappedModel, sField, bp)
}

// GetRelatedHandlers returns all handler functions for the JSONAPI 'GET RELATED' relationship fields for given model.
func (h *Creator) GetRelatedHandlers(model interface{}, basePath ...string) map[*mapping.StructField]http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	handlers := make(map[*mapping.StructField]http.HandlerFunc)
	var bp string
	if len(basePath) != 0 {
		bp = basePath[0]
	}
	for _, relation := range mappedModel.RelationFields() {
		handlers[relation] = h.handleGetRelated(mappedModel, relation, bp)
	}
	return handlers
}

func (h *Creator) handleGetRelated(model *mapping.ModelStruct, field *mapping.StructField, basePath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		// Check the URL 'id' value.
		id := CtxMustGetID(ctx)
		if id == "" {
			log.Debugf("[GET-RELATED][%s] Empty id params", model.Collection())
			err := errors.ErrBadRequest()
			err.Detail = "Provided empty 'id' in url"
			h.marshalErrors(rw, req, 0, err)
			return
		}

		idValue, err := model.Primary().ValueFromString(id)
		if err != nil {
			log.Debugf("[GET-RELATED][%s] Invalid URL id value: '%s': '%v'", model.Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		// check the fieldset for the relatedScope
		relatedScope := query.NewModelC(h.c, field.Relationship().Struct(), field.Kind() == mapping.KindRelationshipMultiple)
		for k, v := range req.URL.Query() {
			if strings.HasPrefix(k, query.ParamFields) {
				if err = h.queryParameterFields(relatedScope, k, v[0]); err != nil {
					log.Debug2f("[GET-RELATED][%s] Setting related fieldset failed: %v", model.Collection(), err)
					h.marshalErrors(rw, req, 0, errors.MapError(err)...)
					return
				}
			}
		}

		// Set preset filters
		s := query.NewModelC(h.c, model, false)
		// Set the primary field value.
		if err = s.FilterField(query.NewFilter(model.Primary(), query.OpEqual, idValue)); err != nil {
			log.Errorf("[GET-RELATED][%s][%s] Adding param primary filter with value: '%s' failed: %v", model.Collection(), field.NeuronName(), id, err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		if err = s.SetFields(field); err != nil {
			log.Errorf("[GET-RELATED][%s][%s] Setting related field into fieldset failed: %v", model.Collection(), field.NeuronName(), err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		if err = s.GetContext(ctx); err != nil {
			log.Debug("[GET-RELATED][SCOPE][%s] Getting /%s/%s root scope failed: %v", s.Struct().Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		linkType := jsonapi.RelatedLink
		// but if the config doesn't allow that - set 'jsonapi.NoLink'
		if !h.MarshalLinks {
			linkType = jsonapi.NoLink
		}

		options := &jsonapi.MarshalOptions{
			Link: jsonapi.LinkOptions{
				Type:         linkType,
				BaseURL:      h.getBasePath(basePath),
				RootID:       id,
				Collection:   model.Collection(),
				RelatedField: field.NeuronName(),
			},
		}

		// get field's value
		v := reflect.ValueOf(s.Value).Elem()
		fieldValue := v.FieldByIndex(field.ReflectField().Index)
		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				relatedScope := query.NewModelC(h.c, field.Relationship().Struct(), field.Kind() == mapping.KindRelationshipMultiple)
				h.marshalScope(relatedScope, rw, req, 200, options)
				return
			}
			fieldValue = fieldValue.Elem()
		}

		switch fieldValue.Kind() {
		case reflect.Slice:
			h.handleGetRelatedMany(ctx, rw, req, s, field, relatedScope, fieldValue, options)
		default:
			h.handleGetRelatedSingle(ctx, rw, req, s, field, relatedScope, fieldValue, options)
		}

	}
}

func (h *Creator) handleGetRelatedSingle(ctx context.Context, rw http.ResponseWriter, req *http.Request, s *query.Scope, field *mapping.StructField, relatedScope *query.Scope, fieldValue reflect.Value, options *jsonapi.MarshalOptions) {
	primary := fieldValue.FieldByIndex(field.Relationship().Struct().Primary().ReflectField().Index).Interface()
	if reflect.DeepEqual(reflect.Zero(field.Relationship().Struct().Primary().ReflectField().Type), primary) {
		relatedScope.Value = nil
		h.marshalScope(relatedScope, rw, req, http.StatusOK, options)
		return
	}

	if err := relatedScope.FilterField(query.NewFilter(relatedScope.Struct().Primary(), query.OpEqual, primary)); err != nil {
		log.Errorf("[GET-RELATED][SCOPE][%s] Adding primary field filter failed: %v. Collection: '%s' field: '%s'", relatedScope.ID(), err, s.Struct().Collection(), field.NeuronName())
		h.marshalErrors(rw, req, 0, errors.ErrInternalError())
		return
	}

	// execute the before getter hook
	if beforeGetHook, ok := Hooks.getHook(s.Struct(), BeforeGetRelated); ok {
		if err := beforeGetHook(ctx, relatedScope); err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}
	}

	if err := relatedScope.GetContext(ctx); err != nil {
		ce, ok := err.(neuronErrors.ClassError)
		if ok {
			if ce.Class() == class.QueryValueNoResult {
				relatedScope.Value = nil
				h.marshalScope(relatedScope, rw, req, http.StatusOK, options)
				return
			}
		}
		h.marshalErrors(rw, req, 0, errors.MapError(err)...)
		return
	}

	// execute the after getter hook
	if afterGetHook, ok := Hooks.getHook(s.Struct(), AfterGetRelated); ok {
		if err := afterGetHook(ctx, relatedScope); err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}
	}

	h.marshalScope(relatedScope, rw, req, http.StatusOK, options)
}

func (h *Creator) handleGetRelatedMany(ctx context.Context, rw http.ResponseWriter, req *http.Request, s *query.Scope, field *mapping.StructField, relatedScope *query.Scope, fieldValue reflect.Value, options *jsonapi.MarshalOptions) {
	primaries := make([]interface{}, 0)

	for i := 0; i < fieldValue.Len(); i++ {
		single := fieldValue.Index(i)
		if single.Kind() == reflect.Ptr {
			if single.IsNil() {
				continue
			}
			single = single.Elem()
		}

		primary := single.FieldByIndex(field.Relationship().Struct().Primary().ReflectField().Index)
		primaries = append(primaries, primary.Interface())
	}

	if len(primaries) == 0 {
		h.marshalScope(relatedScope, rw, req, 200, options)
		return
	}

	if err := relatedScope.FilterField(query.NewFilter(relatedScope.Struct().Primary(), query.OpIn, primaries...)); err != nil {
		log.Errorf("[GET-RELATED][SCOPE][%s] Adding related primary field filter failed: %s. Collection: '%s' field: '%s'", relatedScope.ID(), err, s.Struct().Collection(), field.NeuronName())
		h.marshalErrors(rw, req, 0, errors.ErrInternalError())
		return
	}

	// execute the before lister hook
	if beforeGetHook, ok := Hooks.getHook(s.Struct(), BeforeGetRelated); ok {
		if err := beforeGetHook(ctx, relatedScope); err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}
	}

	if err := relatedScope.ListContext(ctx); err != nil {
		ce, ok := err.(neuronErrors.ClassError)
		if ok {
			if ce.Class() == class.QueryValueNoResult {
				h.marshalScope(relatedScope, rw, req, http.StatusOK, options)
				return
			}
		}
		h.marshalErrors(rw, req, 0, errors.MapError(err)...)
		return
	}

	// execute the after lister hook
	if afterGetHook, ok := Hooks.getHook(s.Struct(), AfterGetRelated); ok {
		if err := afterGetHook(ctx, relatedScope); err != nil {
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}
	}
	h.marshalScope(relatedScope, rw, req, http.StatusOK, options)
}
