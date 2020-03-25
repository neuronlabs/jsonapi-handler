package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/neuronlabs/neuron-core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/config"
	"github.com/neuronlabs/neuron-core/query"
	mocks "github.com/neuronlabs/neuron-mocks"
)

// TestHandleDelete tests handleDelete function.
func TestHandleDelete(t *testing.T) {
	c, err := neuron.NewController(config.Default())
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &config.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{}, HookChecker{})
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {
		h := NewC(c)

		req, err := http.NewRequest("DELETE", "/houses/3", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "3"))

		repo, err := c.GetRepository(House{})
		require.NoError(t, err)

		housesRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
		housesRepo.On("Delete", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaryFilters := s.PrimaryFilters
			if assert.Len(t, primaryFilters, 1) {
				filter := primaryFilters[0]
				if assert.Len(t, filter.Values, 1) {
					v := filter.Values[0]
					assert.Equal(t, query.OpIn, v.Operator)
					if assert.Len(t, v.Values, 1) {
						assert.Equal(t, 3, v.Values[0])
					}
				}
			}
		}).Return(nil)
		housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

		resp := httptest.NewRecorder()
		h.Delete(House{}).ServeHTTP(resp, req)
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})

	t.Run("NotFound", func(t *testing.T) {
		h := NewC(c)
		req, err := http.NewRequest("DELETE", "/houses/3", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "3"))

		repo, err := c.GetRepository(House{})
		require.NoError(t, err)

		housesRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
		housesRepo.On("Delete", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaryFilters := s.PrimaryFilters
			if assert.Len(t, primaryFilters, 1) {
				filter := primaryFilters[0]
				if assert.Len(t, filter.Values, 1) {
					v := filter.Values[0]
					assert.Equal(t, query.OpIn, v.Operator)
					if assert.Len(t, v.Values, 1) {
						assert.Equal(t, 3, v.Values[0])
					}
				}
			}
		}).Return(errors.New(class.QueryValueNoResult, "nothing to delete"))
		housesRepo.On("Rollback", mock.Anything, mock.Anything).Once().Return(nil)

		resp := httptest.NewRecorder()
		h.Delete(House{}).ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)

		payload, err := jsonapi.UnmarshalErrors(resp.Body)
		require.NoError(t, err)

		if assert.Len(t, payload.Errors, 1) {
			jsonapiError := payload.Errors[0]
			code, err := strconv.ParseInt(jsonapiError.Code, 16, 32)
			require.NoError(t, err)

			assert.Equal(t, class.QueryValueNoResult, errors.Class(code))
		}
	})

	t.Run("Hooks", func(t *testing.T) {
		h := NewC(c)

		RegisterHookC(c, HookChecker{}, BeforeDelete, hookCheckerBeforeDelete)
		RegisterHookC(c, HookChecker{}, AfterDelete, hookCheckerAfterDelete)
		req, err := http.NewRequest("DELETE", "/hook_checkers/3", nil)
		require.NoError(t, err)
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "3"))

		req.Header.Add("Accept", jsonapi.MediaType)

		repo, err := c.GetRepository(HookChecker{})
		require.NoError(t, err)

		hooksCheckerRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		hooksCheckerRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
		hooksCheckerRepo.On("Delete", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaryFilters := s.PrimaryFilters
			if assert.Len(t, primaryFilters, 1) {
				filter := primaryFilters[0]
				if assert.Len(t, filter.Values, 1) {
					v := filter.Values[0]
					assert.Equal(t, query.OpIn, v.Operator)
					if assert.Len(t, v.Values, 1) {
						assert.Equal(t, 3, v.Values[0])
					}
				}
			}

			_, ok = s.StoreGet("BD")
			assert.True(t, ok)

		}).Return(nil)
		hooksCheckerRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

		resp := httptest.NewRecorder()
		h.Delete(HookChecker{}).ServeHTTP(resp, req)

		assert.Equal(t, http.StatusNoContent, resp.Code)
	})
}
