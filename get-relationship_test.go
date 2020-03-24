package handler

import (
	"context"
	"io/ioutil"
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

// TestHandleGetRelationship tests the jsonapi handleGetRelationship functions.
func TestHandleGetRelationship(t *testing.T) {
	c, err := neuron.NewController(config.Default())
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &config.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{})
	require.NoError(t, err)

	t.Run("RootNotFound", func(t *testing.T) {
		h := NewC(c)

		req, err := http.NewRequest("GET", "/houses/1/relationships/owner", nil)
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

			_, ok = s.Value.(*House)
			require.True(t, ok)
		}).Return(errors.New(class.QueryValueNoResult, "no result"))

		resp := httptest.NewRecorder()
		h.GetRelationship(House{}, "owner").ServeHTTP(resp, req)

		housesRepo.AssertNumberOfCalls(t, "Get", 1)

		// the status should be 404.
		require.Equal(t, http.StatusNotFound, resp.Code)

		if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
			payload, err := jsonapi.UnmarshalErrors(resp.Body)
			require.NoError(t, err)

			if assert.Len(t, payload.Errors, 1) {
				e := payload.Errors[0]
				code, err := strconv.ParseInt(e.Code, 16, 32)
				require.NoError(t, err)

				assert.Equal(t, class.QueryValueNoResult, errors.Class(code))
			}
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Run("Many", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/humen/1/relationships/houses", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			repo, err := c.GetRepository(Human{})
			require.NoError(t, err)

			humanRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			humanRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*Human)
				require.True(t, ok)

				v.ID = 1
			}).Return(nil)

			repo, err = c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Return(errors.New(class.QueryValueNoResult, "not found"))

			resp := httptest.NewRecorder()
			h.GetRelationship(Human{}, "houses").ServeHTTP(resp, req)

			humanRepo.AssertNumberOfCalls(t, "Get", 1)
			housesRepo.AssertNumberOfCalls(t, "List", 1)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				houses := make([]*House, 0)
				err = jsonapi.UnmarshalC(c, resp.Body, &houses)
				require.NoError(t, err)

				assert.Empty(t, houses)
			}
		})

		t.Run("Single", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses/1/relationships/owner", nil)
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
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.GetRelationship(House{}, "owner").ServeHTTP(resp, req)
			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				data, err := ioutil.ReadAll(resp.Body)
				require.NoError(t, err)

				assert.Contains(t, string(data), "\"data\":null")
			}
		})
	})

	t.Run("Valid", func(t *testing.T) {
		t.Run("Single", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses/1/relationships/owner", nil)
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
				v.OwnerID = 3
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.GetRelationship(House{}, "owner").ServeHTTP(resp, req)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				human := Human{}
				err = jsonapi.UnmarshalC(c, resp.Body, &human)
				require.NoError(t, err)

				assert.Equal(t, 3, human.ID)
			}
		})

		t.Run("Many", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/humen/1/relationships/houses", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")
			req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

			repo, err := c.GetRepository(Human{})
			require.NoError(t, err)

			humanRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			humanRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*Human)
				require.True(t, ok)

				primary := s.PrimaryFilters
				for _, ff := range primary {
					values := ff.Values
					if assert.Len(t, values, 1) {
						ov := values[0]
						assert.Equal(t, query.OpEqual, ov.Operator)
						assert.Contains(t, ov.Values, 1)
					}
				}

				v.ID = 1
			}).Return(nil)

			repo, err = c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			// list sarah houses
			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)

				foreign := s.ForeignFilters
				for _, ff := range foreign {
					values := ff.Values
					if assert.Len(t, values, 1) {
						ov := values[0]
						assert.Equal(t, query.OpEqual, ov.Operator)
						assert.Contains(t, ov.Values, 1)
					}
				}

				*v = append(*v, &House{ID: 1}, &House{ID: 3})
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.GetRelationship(Human{}, "houses").ServeHTTP(resp, req)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				houses := make([]*House, 0)
				err = jsonapi.UnmarshalC(c, resp.Body, &houses)
				require.NoError(t, err)

				if assert.Len(t, houses, 2) {
					var is1, is3 bool
					for _, house := range houses {
						switch house.ID {
						case 1:
							is1 = true
						case 3:
							is3 = true
						default:
							t.Errorf("Invalid houseID: %v", house.ID)
						}
					}
					assert.True(t, is1 && is3)
				}
			}
		})
	})
}
