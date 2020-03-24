package handler

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neuronlabs/neuron-core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	cconfig "github.com/neuronlabs/neuron-core/config"
	"github.com/neuronlabs/neuron-core/query"
	mocks "github.com/neuronlabs/neuron-mocks"
)

// TestHandlePatch tests handlePatch function.
func TestHandlePatch(t *testing.T) {
	c, err := neuron.NewController(cconfig.Default())
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &cconfig.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{}, HookChecker{})
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {
		t.Run("WithContent", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("PATCH", "/houses/1", strings.NewReader(`{"data":{"type":"houses","id": "1", "attributes":{"address":"Some"}}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
			housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				primaries := s.PrimaryFilters
				if assert.Len(t, primaries, 1) {
					pf := primaries[0]
					if assert.Len(t, pf.Values, 1) {
						pv := pf.Values[0]

						assert.Equal(t, query.OpIn, pv.Operator)
						assert.Contains(t, pv.Values, 1)
					}
				}

				v, ok := s.Value.(*House)
				require.True(t, ok)

				assert.Equal(t, "Some", v.Address)
			}).Return(nil)
			housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

			// get result
			housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				primaries := s.PrimaryFilters
				if assert.Len(t, primaries, 1) {
					pf := primaries[0]
					if assert.Len(t, pf.Values, 1) {
						pv := pf.Values[0]

						assert.Equal(t, query.OpEqual, pv.Operator)
						assert.Contains(t, pv.Values, 1)
					}
				}

				v, ok := s.Value.(*House)
				require.True(t, ok)
				v.ID = 1
				v.Address = "Some"
			}).Return(nil)

			resp := httptest.NewRecorder()

			h.Patch(House{}).ServeHTTP(resp, req)

			if !housesRepo.AssertCalled(t, "Patch", mock.Anything, mock.Anything) {
				t.FailNow()
			}

			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type"))

			data, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, `{"links":{"self":"/houses/1"},"data":{"type":"houses","id":"1","attributes":{"address":"Some"},"relationships":{"owner":{"data":null,"links":{"related":"/houses/1/owner","self":"/houses/1/relationships/owner"}}},"links":{"self":"/houses/1"}}}`+"\n", string(data))
		})

		t.Run("NoAccept", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("PATCH", "/houses/1", strings.NewReader(`{"data":{"type":"houses","id": "1", "attributes":{"address":"Some"}}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))
			// req.Header.Add("Accept", jsonapi.MediaType)

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
			housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*House)
				require.True(t, ok)

				assert.Equal(t, "Some", v.Address)
			}).Return(nil)
			housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

			resp := httptest.NewRecorder()
			h.Patch(House{}).ServeHTTP(resp, req)

			if !housesRepo.AssertCalled(t, "Patch", mock.Anything, mock.Anything) {
				t.FailNow()
			}

			assert.Equal(t, http.StatusNoContent, resp.Code)
		})
	})

	t.Run("Error", func(t *testing.T) {
		t.Run("InvalidID", func(t *testing.T) {
			type idcase struct {
				name string
				url  string
			}

			cases := []idcase{{"Empty", " "}, {"InvalidType", "ff1231-ef1213"}}

			for _, cs := range cases {
				t.Run(cs.name, func(t *testing.T) {
					h := NewC(c)

					req, err := http.NewRequest("PATCH", "/houses/"+cs.url, strings.NewReader(`{"data":{"type":"houses","id":"3"}}`))
					require.NoError(t, err)

					req.Header.Add("Content-Type", jsonapi.MediaType)
					req.Header.Add("Accept", jsonapi.MediaType)
					req.Header.Add("Accept-Encoding", "identity")
					req = req.WithContext(context.WithValue(context.Background(), IDKey, cs.url))

					resp := httptest.NewRecorder()
					h.Patch(House{}).ServeHTTP(resp, req)

					assert.Equal(t, http.StatusBadRequest, resp.Code)
				})
			}
		})

		t.Run("IDMismatch", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("PATCH", "/houses/1", strings.NewReader(`{"data":{"type":"houses","id":"3"}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			resp := httptest.NewRecorder()
			h.Patch(House{}).ServeHTTP(resp, req)

			assert.Equal(t, http.StatusConflict, resp.Code)
			data, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Contains(t, string(data), "URL id value: '1' doesn't match input data id value: '3'")
		})

		t.Run("TypeMismatch", func(t *testing.T) {
			// the id in the url mismatches id in the body
			h := NewC(c)

			req, err := http.NewRequest("PATCH", "/houses/1", strings.NewReader(`{"data":{"type":"humen","id":"3"}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			resp := httptest.NewRecorder()
			h.Patch(House{}).ServeHTTP(resp, req)

			assert.Equal(t, http.StatusConflict, resp.Code)
		})

		t.Run("NotFound", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("PATCH", "/houses/1", strings.NewReader(`{"data":{"type":"houses","id": "1", "attributes":{"address":"Some"}}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			// req.Header.Add("Accept", jsonapi.MediaType)

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
			housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*House)
				require.True(t, ok)

				assert.Equal(t, "Some", v.Address)
			}).Return(errors.New(class.QueryValueNoResult, "no result"))
			housesRepo.On("Rollback", mock.Anything, mock.Anything).Once().Return(nil)

			resp := httptest.NewRecorder()
			h.Patch(House{}).ServeHTTP(resp, req)

			if !housesRepo.AssertCalled(t, "Patch", mock.Anything, mock.Anything) {
				t.FailNow()
			}

			assert.Equal(t, http.StatusNotFound, resp.Code)
		})
	})

	t.Run("Hooks", func(t *testing.T) {
		h := NewC(c)
		RegisterHookC(c, HookChecker{}, BeforePatch, hookCheckerBeforePatch)
		RegisterHookC(c, HookChecker{}, AfterPatch, hookCheckerAfterPatch)
		req, err := http.NewRequest("PATCH", "/hook_checkers/1", strings.NewReader(`{"data":{"type":"hook_checkers","id": "1", "attributes":{"number":1}}}`))
		require.NoError(t, err)

		req.Header.Add("Content-Type", jsonapi.MediaType)
		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		repo, err := c.GetRepository(HookChecker{})
		require.NoError(t, err)

		hookCheckerRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		hookCheckerRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
		hookCheckerRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaries := s.PrimaryFilters
			if assert.Len(t, primaries, 1) {
				pf := primaries[0]
				if assert.Len(t, pf.Values, 1) {
					pv := pf.Values[0]

					assert.Equal(t, query.OpIn, pv.Operator)
					assert.Contains(t, pv.Values, 1)
				}
			}

			v, ok := s.Value.(*HookChecker)
			require.True(t, ok)

			assert.Equal(t, 1, v.Number)
			assert.True(t, v.Before)
			assert.False(t, v.After)
		}).Return(nil)
		hookCheckerRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

		// get result
		hookCheckerRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaries := s.PrimaryFilters
			if assert.Len(t, primaries, 1) {
				pf := primaries[0]
				if assert.Len(t, pf.Values, 1) {
					pv := pf.Values[0]

					assert.Equal(t, query.OpEqual, pv.Operator)
					assert.Contains(t, pv.Values, 1)
				}
			}

			v, ok := s.Value.(*HookChecker)
			require.True(t, ok)
			v.ID = 1
			v.Before = true
			v.Number = 3
			f, ok := s.Struct().Field("After")
			if ok {
				err = s.SetFields(f)
				require.NoError(t, err)
			}
		}).Return(nil)

		resp := httptest.NewRecorder()
		h.Patch(HookChecker{}).ServeHTTP(resp, req)

		if !hookCheckerRepo.AssertCalled(t, "Patch", mock.Anything, mock.Anything) {
			t.FailNow()
		}

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type"))

		hc := HookChecker{}
		err = jsonapi.UnmarshalC(c, resp.Body, &hc)
		require.NoError(t, err)

		assert.Equal(t, 1, hc.ID)
		assert.True(t, hc.Before)
		assert.Equal(t, 3, hc.Number)

	})
}
