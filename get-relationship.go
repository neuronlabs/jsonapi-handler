package handler

import (
	"net/http"
	"reflect"

	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	"github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// GetRelationship returns http.HandlerFunc for the JSONAPI get Relationship endpoint for the 'model'
// and relationship 'field'.
func (h *Creator) GetRelationship(model interface{}, field string) http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	sField, ok := mappedModel.RelationField(field)
	if !ok {
		log.Panicf("Field: '%s' not found for the model: '%s'", mappedModel.String())
	}
	return h.handleGetRelationship(mappedModel, sField, "")
}

// GetRelationShipHandlers returns mapping of 'model' relationship fields to related http.HandlerFunc
// for the JSONAPI get relationship endpoints.
func (h *Creator) GetRelationShipHandlers(model interface{}, basePath ...string) map[*mapping.StructField]http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	handlers := make(map[*mapping.StructField]http.HandlerFunc)
	var bp string
	if len(basePath) > 0 {
		bp = basePath[0]
	}
	for _, relation := range mappedModel.RelationFields() {
		handlers[relation] = h.handleGetRelationship(mappedModel, relation, bp)
	}
	return handlers
}

func (h *Creator) handleGetRelationship(model *mapping.ModelStruct, field *mapping.StructField, basePath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		// Check the URL 'id' value.
		id := CtxMustGetID(ctx)
		if id == "" {
			log.Debugf("[GET-RELATIONSHIP][%s] Empty id params", model.Collection())
			err := errors.ErrBadRequest()
			err.Detail = "Provided empty 'id' in url"
			h.marshalErrors(rw, req, 0, err)
			return
		}

		idValue, err := model.Primary().ValueFromString(id)
		if err != nil {
			log.Debugf("[GET-RELATIONSHIP][%s] Invalid URL id value: '%s': '%v'", model.Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		s := query.NewModelC(h.c, model, false)
		if err = s.FilterField(query.NewFilter(model.Primary(), query.OpEqual, idValue)); err != nil {
			log.Errorf("[GET-RELATIONSHIP][SCOPE][%s] Adding param primary filter with value: '%s' failed: %v", s.ID(), id, err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		if err := s.SetFields(field); err != nil {
			log.Errorf("[GET-RELATIONSHIP][SCOPE][%s][%s] Setting related field: '%s' into fieldset failed: %v", s.ID(), s.Struct().Collection(), field.NeuronName(), err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}
		if beforeGetHook, ok := Hooks.getHook(model, BeforeGetRelationship); ok {
			if err := beforeGetHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		if err = s.GetContext(req.Context()); err != nil {
			log.Debugf("[GET-RELATIONSHIP][SCOPE][%s] Getting /%s/%s root scope failed: %v", s.Struct().Collection(), s.Struct().Collection(), id, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		if afterGetHook, ok := Hooks.getHook(model, AfterGetRelationship); ok {
			if err := afterGetHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
				return
			}
		}

		// get field's value
		v := reflect.ValueOf(s.Value).Elem()
		fieldValue := v.FieldByIndex(field.ReflectField().Index)
		if fieldValue.Kind() != reflect.Ptr {
			fieldValue = fieldValue.Addr()
		}

		var relationshipScope *query.Scope
		if fieldValue.IsNil() {
			relationshipScope = query.NewModelC(h.c, field.Relationship().Struct(), field.Kind() == mapping.KindRelationshipMultiple)
			relationshipScope.Value = nil
		} else {
			relationshipScope, err = query.NewC(h.c, fieldValue.Interface())
			if err != nil {
				log.Errorf("Can't create relationship scope: %v", err)
				h.marshalErrors(rw, req, 0, errors.ErrInternalError())
				return
			}
		}

		if err := relationshipScope.SetFieldset(relationshipScope.Struct().Primary()); err != nil {
			log.Errorf("Setting relationship scope primary field's fieldset failed: %v", err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		linkType := jsonapi.RelationshipLink
		// but if the config doesn't allow that - set 'jsonapi.NoLink'
		if !h.MarshalLinks {
			linkType = jsonapi.NoLink
		}

		options := &jsonapi.MarshalOptions{Link: jsonapi.LinkOptions{
			Type:         linkType,
			BaseURL:      h.getBasePath(basePath),
			RootID:       id,
			Collection:   model.Collection(),
			RelatedField: field.NeuronName(),
		}}
		h.marshalScope(relationshipScope, rw, req, http.StatusOK, options)
	}
}
