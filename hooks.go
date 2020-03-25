package handler

import (
	"context"

	"github.com/neuronlabs/neuron-core/controller"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"
)

/** CREATE */

// HookType defines the type of the JSONAPI endpoint.
type HookType int

// hook types enum definitions.
const (
	AfterCreate HookType = iota
	BeforeCreate
	AfterGet
	BeforeGet
	AfterGetRelated
	BeforeGetRelated
	AfterGetRelationship
	BeforeGetRelationship
	AfterList
	BeforeList
	AfterPatch
	BeforePatch
	AfterPatchRelationship
	AfterPatchRelationshipGet
	BeforePatchRelationship
	AfterDelete
	BeforeDelete
)

// Hooks is the storage for the model endpoint hooks.
var Hooks HooksStore = make(map[*mapping.ModelStruct][]HookFunction)

// RegisterHookC registers a 'hook' for given 'model' provided 'endpoint' and a controller 'c'.
func RegisterHookC(c *controller.Controller, model interface{}, endpoint HookType, hook HookFunction) {
	registerHookC(c, model, endpoint, hook)
}

// RegisterHook registers a 'hook' for given 'model' and provided 'endpoint'. The function uses default neuron controller.
func RegisterHook(model interface{}, endpoint HookType, hook HookFunction) {
	registerHookC(controller.Default(), model, endpoint, hook)
}

func registerHookC(c *controller.Controller, model interface{}, endpoint HookType, hook HookFunction) {
	mappedModel := c.MustGetModelStruct(model)
	Hooks.registerHook(mappedModel, endpoint, hook)
}

// HookFunction is the function type used as the hooks for the JSONAPI handlers.
type HookFunction func(ctx context.Context, s *query.Scope) error

// HooksStore is the store for the hooks for given models. For each model a slice of hook functions
// that stores the hooks indexed by the enum value of related HookType.
type HooksStore map[*mapping.ModelStruct][]HookFunction

func (h HooksStore) getHook(model *mapping.ModelStruct, endpoint HookType) (HookFunction, bool) {
	endpointHooks, ok := h[model]
	if !ok {
		return nil, false
	}
	hook := endpointHooks[endpoint]
	return hook, hook != nil
}

func (h HooksStore) registerHook(model *mapping.ModelStruct, endpoint HookType, hook HookFunction) {
	endpoints, ok := h[model]
	if !ok {
		endpoints = make([]HookFunction, BeforeDelete+1)
	}
	endpoints[endpoint] = hook
	h[model] = endpoints
}
