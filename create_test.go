package handler

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/neuronlabs/neuron-core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/neuronlabs/brotli"
	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/config"
	"github.com/neuronlabs/neuron-core/query"
	mocks "github.com/neuronlabs/neuron-mocks"
)

// TestHandleCreate test the handleCreate function.
func TestHandleCreate(t *testing.T) {
	c, err := neuron.NewController(config.Default())
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &config.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{}, HookChecker{})
	require.NoError(t, err)

	t.Run("InvalidInput", func(t *testing.T) {
		h := NewC(c)
		t.Run("Collection", func(t *testing.T) {
			// The collection doesn't match
			req, err := http.NewRequest("POST", "/houses", strings.NewReader(`{"data":{"type":"humen","id":"3"}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			resp := httptest.NewRecorder()

			h.Create(House{}).ServeHTTP(resp, req)

			assert.Equal(t, http.StatusConflict, resp.Code)

			jsonapiErrors, err := jsonapi.UnmarshalErrors(resp.Body)
			require.NoError(t, err)

			if assert.Len(t, jsonapiErrors.Errors, 1) {
				err := jsonapiErrors.Errors[0]
				if assert.NotEqual(t, "", err.Code) {
					code, err := strconv.ParseInt(err.Code, 16, 32)
					require.NoError(t, err)

					assert.Equal(t, class.EncodingUnmarshalCollection, errors.Class(code))
				}
			}
		})

		t.Run("Format", func(t *testing.T) {
			// invalid input -> no closing bracket
			req, err := http.NewRequest("POST", "/houses", strings.NewReader(`{"data":{"type":"houses","id":"3"}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			resp := httptest.NewRecorder()
			h.Create(House{}).ServeHTTP(resp, req)

			assert.Equal(t, http.StatusBadRequest, resp.Code)

			jsonapiErrors, err := jsonapi.UnmarshalErrors(resp.Body)
			require.NoError(t, err)

			if assert.Len(t, jsonapiErrors.Errors, 1) {
				err := jsonapiErrors.Errors[0]
				if assert.NotEqual(t, "", err.Code) {
					code, err := strconv.ParseInt(err.Code, 16, 32)
					require.NoError(t, err)

					assert.Equal(t, class.EncodingUnmarshalInvalidFormat, errors.Class(code))
				}
			}
		})
	})

	t.Run("Valid", func(t *testing.T) {
		type encoding struct {
			Name  string
			Value string
		}

		h := NewC(c)
		encodings := []encoding{{"NoEncoding", ""}, {"Gzip", "gzip"}, {"Deflate", "deflate"}, {"Brotli", "br"}}

		for _, encoding := range encodings {
			t.Run(encoding.Name, func(t *testing.T) {
				req, err := http.NewRequest("POST", "/houses", strings.NewReader(`{"data":{"type":"houses","attributes":{"address":"Some"}}}`))
				require.NoError(t, err)

				req.Header.Add("Content-Type", jsonapi.MediaType)
				req.Header.Add("Accept", jsonapi.MediaType)
				if encoding.Value != "" {
					req.Header.Add("Accept-Encoding", encoding.Value)
				} else {
					req.Header.Add("Accept-Encoding", "identity")
				}

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("Create", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					v, ok := s.Value.(*House)
					require.True(t, ok)

					v.ID = 1
					assert.Equal(t, "Some", v.Address)
				}).Return(nil)
				housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

				resp := httptest.NewRecorder()

				h.Create(House{}).ServeHTTP(resp, req)

				if !housesRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything) {
					t.FailNow()
				}

				assert.Equal(t, 201, resp.Code)
				assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type"))
				rwEncoding := resp.Header().Get("Content-Encoding")
				assert.Equal(t, encoding.Value, rwEncoding)

				var reader io.Reader
				require.Equal(t, encoding.Value, rwEncoding)

				switch rwEncoding {
				case "gzip":
					reader, err = gzip.NewReader(resp.Body)
					require.NoError(t, err)
				case "deflate":
					reader = flate.NewReader(resp.Body)
				case "br":
					reader = brotli.NewReader(resp.Body)
				default:
					reader = resp.Body
				}

				data, err := ioutil.ReadAll(reader)
				require.NoError(t, err)

				assert.Equal(t, `{"data":{"type":"houses","id":"1","attributes":{"address":"Some"}}}`+"\n", string(data))
			})
		}
	})

	t.Run("WithLinks", func(t *testing.T) {
		h := NewC(c)
		h.MarshalLinks = true

		req, err := http.NewRequest("POST", "/houses", strings.NewReader(`{"data":{"type":"houses","attributes":{"address":"Some"}}}`))
		require.NoError(t, err)

		req.Header.Add("Content-Type", jsonapi.MediaType)
		req.Header.Add("Accept", jsonapi.MediaType)

		req.Header.Add("Accept-Encoding", "identity")

		repo, err := c.GetRepository(House{})
		require.NoError(t, err)

		housesRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
		housesRepo.On("Create", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			v, ok := s.Value.(*House)
			require.True(t, ok)

			v.ID = 1
			assert.Equal(t, "Some", v.Address)
		}).Return(nil)
		housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

		resp := httptest.NewRecorder()

		h.Create(House{}).ServeHTTP(resp, req)

		if !housesRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything) {
			t.FailNow()
		}

		assert.Equal(t, 201, resp.Code)
		assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type"))
		data, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, `{"links":{"self":"/houses/1"},"data":{"type":"houses","id":"1","attributes":{"address":"Some"},"links":{"self":"/houses/1"}}}`+"\n", string(data))
	})

	t.Run("ClientID", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("POST", "/houses", strings.NewReader(`{"data":{"type":"houses","id": "3", "attributes":{"address":"Some"}}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
			housesRepo.On("Create", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*House)
				require.True(t, ok)

				assert.Equal(t, "Some", v.Address)
			}).Return(nil)
			housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

			resp := httptest.NewRecorder()

			h.Create(House{}).ServeHTTP(resp, req)
			if !housesRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything) {
				t.FailNow()
			}

			assert.Equal(t, 201, resp.Code)
			assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type"))

			data, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, `{"data":{"type":"houses","id":"3","attributes":{"address":"Some"}}}`+"\n", string(data))
		})

		t.Run("Duplicate", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("POST", "/houses", strings.NewReader(`{"data":{"type":"houses","id": "3", "attributes":{"address":"Some"}}}`))
			require.NoError(t, err)

			req.Header.Add("Content-Type", jsonapi.MediaType)
			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
			housesRepo.On("Create", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*House)
				require.True(t, ok)

				assert.Equal(t, "Some", v.Address)
			}).Return(errors.New(class.QueryViolationUnique, "duplicated value"))
			housesRepo.On("Rollback", mock.Anything, mock.Anything).Once().Return(nil)

			resp := httptest.NewRecorder()

			h.Create(House{}).ServeHTTP(resp, req)

			if !housesRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything) {
				t.FailNow()
			}

			require.Equal(t, 409, resp.Code)
			assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type"))

			errs, err := jsonapi.UnmarshalErrors(resp.Body)
			require.NoError(t, err)

			if assert.Len(t, errs.Errors, 1) {
				jsonapiError := errs.Errors[0]

				code, err := strconv.ParseInt(jsonapiError.Code, 16, 32)
				require.NoError(t, err)

				assert.Equal(t, class.QueryViolationUnique, errors.Class(code))
			}
		})
	})

	t.Run("Hooks", func(t *testing.T) {
		h := NewC(c)

		RegisterHookC(c, HookChecker{}, BeforeCreate, hookCheckerBeforeCreateAPI)
		RegisterHookC(c, HookChecker{}, AfterCreate, hookCheckerAfterCreateAPI)

		req, err := http.NewRequest("POST", "/hook_checkers", strings.NewReader(`{"data":{"type":"hook_checkers"}}`))
		require.NoError(t, err)

		req.Header.Add("Content-Type", jsonapi.MediaType)
		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")

		repo, err := c.GetRepository(HookChecker{})
		require.NoError(t, err)

		hcRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		hcRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
		hcRepo.On("Create", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			v, ok := s.Value.(*HookChecker)
			require.True(t, ok)

			v.ID = 1

			assert.True(t, v.Before)
			assert.False(t, v.After)
		}).Return(nil)
		hcRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)

		resp := httptest.NewRecorder()

		h.Create(HookChecker{}).ServeHTTP(resp, req)

		hc := HookChecker{}
		buf := &bytes.Buffer{}
		tee := io.TeeReader(resp.Body, buf)
		err = jsonapi.UnmarshalC(c, tee, &hc)
		require.NoError(t, err, buf.String())

		assert.Equal(t, 1, hc.ID)
		assert.True(t, hc.Before)
		assert.True(t, hc.After, buf.String())
	})
}
