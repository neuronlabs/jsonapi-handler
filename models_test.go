package handler

import (
	"context"
	"fmt"

	"github.com/neuronlabs/neuron-core/query"

	"github.com/neuronlabs/jsonapi-handler/log"
)

// House is the model used by the jsonapi handler tests.
type House struct {
	ID      int `neuron:"type=primary;flags=client-id"`
	Address string
	Owner   *Human
	OwnerID int `neuron:"type=fk"`
}

// Human is the model used by the jsonapi handler tests.
type Human struct {
	ID     int
	Name   string
	Age    int
	Houses []*House `neuron:"foreign=OwnerID"`
}

// Car is the model used by the jsonapi handler tests.
type Car struct {
	ID      int
	Brand   string
	Owner   *Human
	OwnerID int `neuron:"type=fk;foreign=Owner"`
}

type HookChecker struct {
	ID     int
	Before bool
	After  bool
	Number int
}

// BeforeCreateAPI implements hook.BeforeCreatorAPI.
func hookCheckerBeforeCreateAPI(ctx context.Context, s *query.Scope) error {
	h, ok := s.Value.(*HookChecker)
	if !ok {
		return fmt.Errorf("ERR: invalid value")
	}
	h.Before = true
	return nil
}

// AfterCreateAPI implements hook.AfterCreatorAPI
func hookCheckerAfterCreateAPI(ctx context.Context, s *query.Scope) error {
	h, ok := s.Value.(*HookChecker)
	if !ok {
		return fmt.Errorf("ERR: invalid value")
	}
	h.After = true
	return nil
}

func hookCheckerBeforeDelete(ctx context.Context, s *query.Scope) error {
	s.StoreSet("BD", true)
	return nil
}

func hookCheckerAfterDelete(ctx context.Context, s *query.Scope) error {
	s.StoreSet("AD", true)
	return nil
}

// BeforeCreateAPI implements hook.BeforeCreatorAPI
func hookCheckerBeforePatch(ctx context.Context, s *query.Scope) error {
	h, ok := s.Value.(*HookChecker)
	if !ok {
		log.Panicf("Invalid HookChecker value: %v", s.Value)
	}
	h.Before = true
	return nil
}

// AfterCreateAPI implements hook.AfterCreatorAPI
func hookCheckerAfterPatch(ctx context.Context, s *query.Scope) error {
	a, ok := s.Struct().Field("after")
	if ok {
		return s.SetFields(a)
	}
	return nil
}

func hookCheckerBeforeGet(ctx context.Context, s *query.Scope) error {
	h, ok := s.Value.(*HookChecker)
	if !ok {
		return fmt.Errorf("ERROR")
	}
	h.Before = true
	return nil
}

func hookCheckerAfterGet(ctx context.Context, s *query.Scope) error {
	h, ok := s.Value.(*HookChecker)
	if !ok {
		return fmt.Errorf("ERROR")
	}
	h.After = true
	return nil
}

func hooksCheckerBeforeList(ctx context.Context, s *query.Scope) error {
	return s.SetFields("before")
}

func hooksCheckerAfterList(ctx context.Context, s *query.Scope) error {
	models := s.Value.(*[]*HookChecker)
	if err := s.SetFields("number"); err != nil {
		return err
	}
	for i, m := range *models {
		m.Number = i
	}
	return nil
}
