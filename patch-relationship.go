package handler

import (
	"net/http"
	"reflect"

	neuronErrors "github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	"github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// PatchRelationship returns JSONAPI patch relationship http.HandlerFunc for given 'model' and it's 'field' relationship.
func (h *Creator) PatchRelationship(model interface{}, field string) http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	sField, ok := mappedModel.RelationField(field)
	if !ok {
		log.Panicf("Model: '%s' doesn't have field: '%s'", mappedModel.String(), field)
	}
	return h.handlePatchRelationship(mappedModel, sField, "")
}

// PatchRelationshipHandlers returns mapping for the 'model' relation fields to related JSONAPI patch relationship http.HandlerFunc.
func (h *Creator) PatchRelationshipHandlers(model interface{}, basePath ...string) map[*mapping.StructField]http.HandlerFunc {
	mappedModel := h.c.MustGetModelStruct(model)
	relationFields := mappedModel.RelationFields()
	handlers := make(map[*mapping.StructField]http.HandlerFunc, len(relationFields))
	if len(relationFields) == 0 {
		return handlers
	}

	// get the base path
	var bp string
	if len(basePath) > 0 {
		bp = basePath[0]
	}
	// set the http.HandlerFunc for each relation field.
	for _, relation := range relationFields {
		handlers[relation] = h.handlePatchRelationship(mappedModel, relation, bp)
	}
	return handlers
}

func (h *Creator) handlePatchRelationship(model *mapping.ModelStruct, field *mapping.StructField, basePath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		sID := CtxMustGetID(ctx)
		id, err := model.Primary().ValueFromString(sID)
		if err != nil {
			log.Debugf("Invalid 'id': '%v' in url: %v", sID, err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		s := query.NewModelC(h.c, model, false)

		var nilData bool
		switch field.Kind() {
		case mapping.KindRelationshipSingle:
			v := reflect.New(field.ReflectField().Type.Elem()).Interface()
			selected, err := jsonapi.UnmarshalWithSelectedC(h.c, req.Body, v, h.jsonapiUnmarshalOptions())
			if err != nil {
				cl, ok := err.(neuronErrors.ClassError)
				if !ok {
					log.Errorf("Unmarshal patch-relationship content failed: %v", err)
					h.marshalErrors(rw, req, 0, errors.MapError(err)...)
					return
				}

				if cl.Class() == class.EncodingUnmarshalNoData {
					nilData = true
					v = nil
				} else {
					log.Errorf("Unmarshal data failed: %v", err)
					h.marshalErrors(rw, req, 0, errors.MapError(err)...)
					return
				}
			}

			if !nilData {
				// check if any other field than primary key were selected.
				for _, singleSelected := range selected {
					if singleSelected.Kind() != mapping.KindPrimary {
						err := errors.ErrInvalidJSONFieldValue()
						err.Detail = "Patching relationship with non primary fields"
						h.marshalErrors(rw, req, 0, err)
						return
					}
				}
				// set the value of 'v' relationship on the scope's single relationship field value
				sv := reflect.ValueOf(s.Value).Elem()
				fieldValue := sv.FieldByIndex(field.ReflectField().Index)
				fieldValue.Set(reflect.ValueOf(v))
			}
		case mapping.KindRelationshipMultiple:
			// get the relationship field slice value
			fieldType := field.ReflectField().Type
			for fieldType.Kind() == reflect.Ptr || fieldType.Kind() == reflect.Slice {
				fieldType = fieldType.Elem()
			}

			value := reflect.New(reflect.SliceOf(reflect.PtrTo(fieldType))).Interface()
			log.Debugf("Value: %T", value)
			err = jsonapi.UnmarshalC(h.c, req.Body, value)
			if err != nil {
				ec, ok := err.(neuronErrors.ClassError)
				if ok && ec.Class() == class.EncodingUnmarshalNoData {
					nilData = true
				} else {
					log.Debugf("Unmarshal patch-relationship content failed: %v", err)
					h.marshalErrors(rw, req, 0, errors.MapError(err)...)
					return
				}
			}

			sv := reflect.ValueOf(s.Value).Elem()

			unmarshaledValue := reflect.ValueOf(value)
			if field.ReflectField().Type.Kind() == reflect.Slice {
				unmarshaledValue = unmarshaledValue.Elem()
			}

			if unmarshaledValue.Len() == 0 {
				nilData = true
			}
			sv.FieldByIndex(field.ReflectField().Index).Set(unmarshaledValue)
		default:
			log.Errorf("Unknown field: '%s' kind: %s", field.NeuronName(), field.Kind())
			h.marshalErrors(rw, req, 500, errors.ErrInternalError())
			return
		}
		if err = s.FilterField(query.NewFilter(model.Primary(), query.OpEqual, id)); err != nil {
			log.Errorf("[PATCH-RELATIONSHIP][SCOPE][%s] Adding param primary filter with value: '%s' failed: %v", s.ID(), id, err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		if err := s.SetFields(field); err != nil {
			log.Errorf("[PATCH-RELATIONSHIP][SCOPE][%s][%s] Setting related field: '%s' into fieldset failed: %v", s.ID(), s.Struct().Collection(), field.NeuronName(), err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}
		if log.Level() == log.LDEBUG3 {
			log.Debug3f("Patching Relationship Scope: %s", s)
		}

		if hookBeforePatch, ok := Hooks.getHook(model, BeforePatchRelationship); ok {
			if err = hookBeforePatch(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			}
		}

		if err = s.PatchContext(ctx); err != nil {
			log.Debugf("[PATCH-RELATIONSHIP][SCOPE][%s] Patching '%s' failed: %v", s.ID(), s.Struct().Collection(), err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		if hookAfterPatch, ok := Hooks.getHook(model, AfterPatchRelationship); ok {
			if err = hookAfterPatch(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			}
		}

		if req.Header.Get("Accept") != jsonapi.MediaType {
			log.Debug3("No Accept Header - response with '204' - http.StatusNoContent")
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		linkType := jsonapi.RelationshipLink
		// but if the config doesn't allow that - set 'jsonapi.NoLink'
		if !h.MarshalLinks {
			linkType = jsonapi.NoLink
		}

		marshalOptions := &jsonapi.MarshalOptions{Link: jsonapi.LinkOptions{
			Type:         linkType,
			BaseURL:      h.getBasePath(basePath),
			RootID:       sID,
			Collection:   model.Collection(),
			RelatedField: field.NeuronName(),
		}}

		if nilData {
			resultScope := query.NewModelC(h.c, field.Relationship().Struct(), field.Kind() == mapping.KindRelationshipMultiple)
			if field.Kind() == mapping.KindRelationshipSingle {
				resultScope.Value = nil
			}
			h.marshalScope(resultScope, rw, req, 200, marshalOptions)
			return
		}

		resultScope := query.NewModelC(h.c, model, false)
		if err = resultScope.FilterField(query.NewFilter(model.Primary(), query.OpEqual, id)); err != nil {
			log.Errorf("[PATCH-RELATIONSHIP][SCOPE][%s] Adding param primary filter to return content scope failed: %v", err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}

		if err = resultScope.SetFieldset(field.NeuronName()); err != nil {
			log.Errorf("[PATCH-RELATIONSHIP][SCOPE] Setting 'id' field to the fieldset of returning 'get' scope fails: %v", err)
			h.marshalErrors(rw, req, 0, errors.ErrInternalError())
			return
		}
		if log.Level().IsAllowed(log.LDEBUG3) {
			log.Debug3f("Getting relationship value: %s", resultScope.String())
		}

		if hookAfterPatch, ok := Hooks.getHook(model, AfterPatchRelationshipGet); ok {
			if err = hookAfterPatch(ctx, resultScope); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			}
		}

		if err = resultScope.GetContext(ctx); err != nil {
			log.Infof("[PATCH][SCOPE][%s] Getting resource after patching failed: %v", err)
			h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			return
		}

		if hookAfterPatch, ok := Hooks.getHook(model, AfterPatchRelationshipGet); ok {
			if err = hookAfterPatch(ctx, resultScope); err != nil {
				h.marshalErrors(rw, req, 0, errors.MapError(err)...)
			}
		}

		relatedField := reflect.ValueOf(resultScope.Value).Elem().FieldByIndex(field.ReflectField().Index)
		if relatedField.Kind() != reflect.Ptr {
			relatedField = relatedField.Addr()
		}
		relationScope, err := query.NewC(h.c, relatedField.Interface())
		if err != nil {
			log.Errorf("Can't created related scope. Model: %s Field: %s", model.Collection(), field.String())
			h.marshalErrors(rw, req, 500, errors.ErrInternalError())
			return
		}
		if err = relationScope.SetFieldset("id"); err != nil {
			log.Errorf("Can't add primary field to patch relation scope fieldset: %v", err)
			h.marshalErrors(rw, req, 500, errors.ErrInternalError())
			return
		}
		h.marshalScope(relationScope, rw, req, http.StatusOK, marshalOptions)
	}
}
