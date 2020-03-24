package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

// TestHandleGet tests the handleGet function.
func TestHandleGet(t *testing.T) {
	c, err := neuron.NewController(config.Default())
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &config.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{}, HookChecker{})
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {
		t.Run("Fieldset", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses/1?fields[houses]=address", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				primaryFilters := s.PrimaryFilters
				if assert.Len(t, primaryFilters, 1) {
					filter := primaryFilters[0]
					if assert.Len(t, filter.Values, 1) {
						v := filter.Values[0]
						assert.Equal(t, query.OpEqual, v.Operator)
						if assert.Len(t, v.Values, 1) {
							assert.Equal(t, 1, v.Values[0])
						}
					}
				}

				assert.Len(t, s.Fieldset, 2)
				assert.Contains(t, s.Fieldset, s.Struct().Primary().NeuronName())
				addressField, ok := s.Struct().Attribute("address")
				require.True(t, ok)
				assert.Contains(t, s.Fieldset, addressField.NeuronName())

				v, ok := s.Value.(*House)
				require.True(t, ok)

				v.ID = 1
				v.Address = "Main Rd 52"
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.Get(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				house := &House{}
				err = jsonapi.UnmarshalC(c, resp.Body, house)
				require.NoError(t, err)

				assert.Equal(t, 1, house.ID)
				assert.Equal(t, "Main Rd 52", house.Address)
				assert.Nil(t, house.Owner)
			}
		})

		t.Run("Include", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses/1?include=owner", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				primaryFilters := s.PrimaryFilters
				if assert.Len(t, primaryFilters, 1) {
					filter := primaryFilters[0]
					if assert.Len(t, filter.Values, 1) {
						v := filter.Values[0]
						assert.Equal(t, query.OpEqual, v.Operator)
						if assert.Len(t, v.Values, 1) {
							assert.Equal(t, 1, v.Values[0])
						}
					}
				}

				assert.Len(t, s.Fieldset, 4)

				assert.Contains(t, s.Fieldset, s.Struct().Primary().NeuronName())
				addressField, ok := s.Struct().Attribute("address")
				require.True(t, ok)
				assert.Contains(t, s.Fieldset, addressField.NeuronName())

				v, ok := s.Value.(*House)
				require.True(t, ok)

				v.ID = 1
				v.Address = "Main Rd 52"
				v.OwnerID = 4
			}).Return(nil)

			repo, err = c.GetRepository(Human{})
			require.NoError(t, err)

			humanRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			for i := 0; i < 2; i++ {
				humanRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					v := s.Value.(*[]*Human)

					*v = append(*v, &Human{ID: 4, Name: "Elisabeth", Age: 88})
				}).Return(nil)
			}

			// list elisabeth houses
			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)

				*v = append(*v, &House{ID: 1}, &House{ID: 5})
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.Get(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				buf := &bytes.Buffer{}

				tee := io.TeeReader(resp.Body, buf)
				house := &House{}
				s, err := jsonapi.UnmarshalSingleScopeC(c, tee, house)
				require.NoError(t, err)

				assert.Equal(t, 1, house.ID)
				assert.Equal(t, "Main Rd 52", house.Address)
				if assert.NotNil(t, house.Owner) {
					assert.Equal(t, 4, house.Owner.ID)
				}

				input := buf.String()
				assert.True(t, strings.Contains(input, "include"), input)

				// Unmarshaling includes should be fixed in neuron-core#22
				t.Skipf("Waiting for NeuronCore#22")

				humanValues, err := s.IncludedModelValues(&Human{})
				require.NoError(t, err)

				if assert.Len(t, humanValues, 1) {
					v, ok := humanValues[house.Owner.ID]
					require.True(t, ok)

					human, ok := v.(*Human)
					require.True(t, ok)

					assert.Equal(t, "Elisabeth", human.Name)
				}
			}
		})

		h := NewC(c)

		req, err := http.NewRequest("GET", "/houses/1", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		repo, err := c.GetRepository(House{})
		require.NoError(t, err)

		housesRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaryFilters := s.PrimaryFilters
			if assert.Len(t, primaryFilters, 1) {
				filter := primaryFilters[0]
				if assert.Len(t, filter.Values, 1) {
					v := filter.Values[0]
					assert.Equal(t, query.OpEqual, v.Operator)
					if assert.Len(t, v.Values, 1) {
						assert.Equal(t, 1, v.Values[0])
					}
				}
			}

			assert.Len(t, s.Fieldset, 4)

			assert.Contains(t, s.Fieldset, s.Struct().Primary().NeuronName())
			addressField, ok := s.Struct().Attribute("address")
			require.True(t, ok)
			assert.Contains(t, s.Fieldset, addressField.NeuronName())

			v, ok := s.Value.(*House)
			require.True(t, ok)

			v.ID = 1
			v.Address = "Main Rd 52"
			v.OwnerID = 4
		}).Return(nil)

		resp := httptest.NewRecorder()
		h.Get(House{}).ServeHTTP(resp, req)

		// the status should be 200.
		require.Equal(t, http.StatusOK, resp.Code)

		if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
			house := &House{}
			err = jsonapi.UnmarshalC(c, resp.Body, house)
			require.NoError(t, err)

			assert.Equal(t, 1, house.ID)
			assert.Equal(t, "Main Rd 52", house.Address)
			if assert.NotNil(t, house.Owner) {
				assert.Equal(t, 4, house.Owner.ID)
			}
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		h := NewC(c)

		req, err := http.NewRequest("GET", "/houses/1", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		repo, err := c.GetRepository(House{})
		require.NoError(t, err)

		housesRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaryFilters := s.PrimaryFilters
			if assert.Len(t, primaryFilters, 1) {
				filter := primaryFilters[0]
				if assert.Len(t, filter.Values, 1) {
					v := filter.Values[0]
					assert.Equal(t, query.OpEqual, v.Operator)
					if assert.Len(t, v.Values, 1) {
						assert.Equal(t, 1, v.Values[0])
					}
				}
			}
		}).Return(errors.New(class.QueryValueNoResult, "not found"))

		resp := httptest.NewRecorder()
		h.Get(House{}).ServeHTTP(resp, req)

		// the status should be 200.
		require.Equal(t, http.StatusNotFound, resp.Code)

		if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
			payload, err := jsonapi.UnmarshalErrors(resp.Body)
			require.NoError(t, err)

			if assert.Len(t, payload.Errors, 1) {
				payloadErr := payload.Errors[0]
				code, err := strconv.ParseInt(payloadErr.Code, 16, 32)
				require.NoError(t, err)

				assert.Equal(t, class.QueryValueNoResult, errors.Class(code))
			}
		}
	})

	t.Run("NoID", func(t *testing.T) {
		h := NewC(c)

		req, err := http.NewRequest("GET", "/houses/ ", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, ""))

		resp := httptest.NewRecorder()
		h.Get(House{}).ServeHTTP(resp, req)

		require.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("Links", func(t *testing.T) {
		t.Run("Invalid", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses/1?links=invalid", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			resp := httptest.NewRecorder()
			h.Get(House{}).ServeHTTP(resp, req)

			require.Equal(t, http.StatusBadRequest, resp.Code)
		})

		t.Run("Valid", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses/1?links=false", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*House)
				require.True(t, ok)

				v.ID = 1
				v.Address = "Main Rd 52"
				v.OwnerID = 4
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.Get(House{}).ServeHTTP(resp, req)

			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				house := &House{}
				err = jsonapi.UnmarshalC(c, resp.Body, house)
				require.NoError(t, err)

				assert.Equal(t, 1, house.ID)
				assert.Equal(t, "Main Rd 52", house.Address)
				if assert.NotNil(t, house.Owner) {
					assert.Equal(t, 4, house.Owner.ID)
				}
			}
		})
	})

	t.Run("InvalidQueryParameter", func(t *testing.T) {
		h := NewC(c)
		h.StrictQueriesMode = true

		req, err := http.NewRequest("GET", "/houses/1?invalid=query", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		resp := httptest.NewRecorder()
		h.Get(House{}).ServeHTTP(resp, req)

		require.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("MultipleQueryValues", func(t *testing.T) {
		h := NewC(c)
		h.StrictQueriesMode = true

		req, err := http.NewRequest("GET", "/houses/1?invalid=query&invalid=parameter", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		resp := httptest.NewRecorder()
		h.Get(House{}).ServeHTTP(resp, req)

		require.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("ErrorLimit", func(t *testing.T) {

		h := NewC(c)
		h.StrictQueriesMode = true
		h.QueryErrorsLimit = 1

		req, err := http.NewRequest("GET", "/houses/1?invalid=query&filter[houses][invalid][$eq]=4", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		resp := httptest.NewRecorder()
		h.Get(House{}).ServeHTTP(resp, req)

		require.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("Hooks", func(t *testing.T) {
		h := NewC(c)
		RegisterHookC(c, HookChecker{}, BeforeGet, hookCheckerBeforeGet)
		RegisterHookC(c, HookChecker{}, AfterGet, hookCheckerAfterGet)

		req, err := http.NewRequest("GET", "/hook_checkers/1", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")
		req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

		repo, err := c.GetRepository(HookChecker{})
		require.NoError(t, err)

		hookCheckersRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		hookCheckersRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			primaryFilters := s.PrimaryFilters
			if assert.Len(t, primaryFilters, 1) {
				filter := primaryFilters[0]
				if assert.Len(t, filter.Values, 1) {
					v := filter.Values[0]
					assert.Equal(t, query.OpEqual, v.Operator)
					if assert.Len(t, v.Values, 1) {
						assert.Equal(t, 1, v.Values[0])
					}
				}
			}

			assert.Len(t, s.Fieldset, 4)
			assert.Contains(t, s.Fieldset, s.Struct().Primary().NeuronName())

			v, ok := s.Value.(*HookChecker)
			require.True(t, ok)

			v.ID = 1
		}).Return(nil)

		resp := httptest.NewRecorder()
		h.Get(HookChecker{}).ServeHTTP(resp, req)

		// the status should be 200.
		require.Equal(t, http.StatusOK, resp.Code)

		if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
			hc := &HookChecker{}
			err = jsonapi.UnmarshalC(c, resp.Body, hc)
			require.NoError(t, err)

			assert.Equal(t, 1, hc.ID)
			assert.True(t, hc.After)
		}
	})
}
