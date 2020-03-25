package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/config"
	"github.com/neuronlabs/neuron-core/query"
	mocks "github.com/neuronlabs/neuron-mocks"
)

// TestHandlePatchRelationship tests the jsonapi handleGetRelationship functions.
func TestHandlePatchRelationship(t *testing.T) {
	cfg := config.Default()

	c, err := neuron.NewController(cfg)
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &config.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{})
	require.NoError(t, err)

	t.Run("RootNotFound", func(t *testing.T) {
		h := NewC(c)

		req, err := http.NewRequest("PATCH", "/houses/1/relationships/owner", strings.NewReader(`{"data":null}`))
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

			pfs := s.PrimaryFilters
			if assert.Len(t, pfs, 1) {
				pfV := pfs[0].Values
				if assert.Len(t, pfV, 1) {
					vs := pfV[0]
					assert.Equal(t, query.OpIn, vs.Operator)
					if assert.Len(t, vs.Values, 1) {
						assert.Equal(t, 1, vs.Values[0])
					}
				}
			}

			_, ok = s.Value.(*House)
			require.True(t, ok)
		}).Return(errors.New(class.QueryValueNoResult, "no result"))
		housesRepo.On("Rollback", mock.Anything, mock.Anything).Once().Return(nil)
		resp := httptest.NewRecorder()
		h.PatchRelationship(House{}, "owner").ServeHTTP(resp, req)

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

	t.Run("Valid", func(t *testing.T) {
		t.Run("Single", func(t *testing.T) {
			t.Run("Clear", func(t *testing.T) {
				h := NewC(c)

				require.NoError(t, err)

				req, err := http.NewRequest("PATCH", "/houses/1/relationships/owner", strings.NewReader(`{"data":null}`))
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

				housesRepo.On("Commit", mock.Anything, mock.Anything).Return(nil)
				housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					pfs := s.PrimaryFilters
					if assert.Len(t, pfs, 1) {
						pfV := pfs[0].Values
						if assert.Len(t, pfV, 1) {
							vs := pfV[0]
							assert.Equal(t, query.OpIn, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					house, ok := s.Value.(*House)
					require.True(t, ok)

					house.ID = 1
					assert.Equal(t, 0, house.OwnerID)
				}).Return(nil)

				housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					pfs := s.PrimaryFilters
					if assert.Len(t, pfs, 1) {
						pfV := pfs[0].Values
						if assert.Len(t, pfV, 1) {
							vs := pfV[0]
							assert.Equal(t, query.OpEqual, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					house, ok := s.Value.(*House)
					require.True(t, ok)

					house.ID = 1
					house.OwnerID = 5
				}).Return(nil)

				resp := httptest.NewRecorder()
				h.PatchRelationship(House{}, "owner").ServeHTTP(resp, req)

				buf := &bytes.Buffer{}
				_, err = buf.ReadFrom(resp.Body)
				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.Code)

				strings.Contains(buf.String(), `"data":null`)
			})

			t.Run("Set", func(t *testing.T) {
				testNames := []string{"WithContent", "NoContent"}
				for i, name := range testNames {
					t.Run(name, func(t *testing.T) {
						h := NewC(c)

						require.NoError(t, err)

						req, err := http.NewRequest("PATCH", "/houses/1/relationships/owner", strings.NewReader(`{"data":{"type": "humen", "id":"4"}}`))
						require.NoError(t, err)

						req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))
						req.Header.Add("Content-Type", jsonapi.MediaType)
						if i == 0 {
							req.Header.Add("Accept", jsonapi.MediaType)
						}
						req.Header.Add("Accept-Encoding", "identity")

						repo, err := c.GetRepository(House{})
						require.NoError(t, err)

						housesRepo, ok := repo.(*mocks.Repository)
						require.True(t, ok)

						housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)

						housesRepo.On("Commit", mock.Anything, mock.Anything).Return(nil)
						housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
							s, ok := args[1].(*query.Scope)
							require.True(t, ok)

							pfs := s.PrimaryFilters
							if assert.Len(t, pfs, 1) {
								pfV := pfs[0].Values
								if assert.Len(t, pfV, 1) {
									vs := pfV[0]
									assert.Equal(t, query.OpIn, vs.Operator)
									if assert.Len(t, vs.Values, 1) {
										assert.Equal(t, 1, vs.Values[0])
									}
								}
							}

							house, ok := s.Value.(*House)
							require.True(t, ok)

							house.ID = 1
							assert.Equal(t, 4, house.OwnerID)
						}).Return(nil)

						housesRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
							s, ok := args[1].(*query.Scope)
							require.True(t, ok)

							pfs := s.PrimaryFilters
							if assert.Len(t, pfs, 1) {
								pfV := pfs[0].Values
								if assert.Len(t, pfV, 1) {
									vs := pfV[0]
									assert.Equal(t, query.OpEqual, vs.Operator)
									if assert.Len(t, vs.Values, 1) {
										assert.Equal(t, 1, vs.Values[0])
									}
								}
							}

							house, ok := s.Value.(*House)
							require.True(t, ok)

							house.ID = 1
							house.OwnerID = 4
						}).Return(nil)

						resp := httptest.NewRecorder()
						h.PatchRelationship(House{}, "owner").ServeHTTP(resp, req)

						switch i {
						case 0:
							buf := &bytes.Buffer{}
							_, err = buf.ReadFrom(resp.Body)
							require.NoError(t, err)

							assert.Equal(t, http.StatusOK, resp.Code)

							strings.Contains(buf.String(), `"data":{"type":"humen", "id":"4"}`)
						case 1:
							assert.Equal(t, http.StatusNoContent, resp.Code)
						}
					})
				}
			})
		})

		t.Run("Many", func(t *testing.T) {
			t.Run("Clear", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("PATCH", "/humen/1/relationships/houses", strings.NewReader(`{"data":[]}`))
				require.NoError(t, err)

				req.Header.Add("Content-Type", jsonapi.MediaType)
				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")
				req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

				repo, err := c.GetRepository(Human{})
				require.NoError(t, err)

				humenRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				humenRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				humenRepo.On("Commit", mock.Anything, mock.Anything).Return(nil)
				humenRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					pfs := s.PrimaryFilters
					if assert.Len(t, pfs, 1) {
						pfV := pfs[0].Values
						if assert.Len(t, pfV, 1) {
							vs := pfV[0]
							assert.Equal(t, query.OpIn, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					humen, ok := s.Value.(*[]*Human)
					require.True(t, ok)

					*humen = append(*humen, &Human{ID: 1})
				}).Return(nil)

				repo, err = c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					ffs := s.ForeignFilters
					if assert.Len(t, ffs, 1) {
						ffV := ffs[0].Values
						assert.Equal(t, "owner_id", ffs[0].StructField.NeuronName())
						if assert.Len(t, ffV, 1) {
							vs := ffV[0]
							assert.Equal(t, query.OpIn, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					houses, ok := s.Value.(*[]*House)
					require.True(t, ok)

					*houses = append(*houses, &House{ID: 3, OwnerID: 1})
				}).Return(nil)

				housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					house, ok := s.Value.(*House)
					require.True(t, ok)

					assert.Equal(t, 0, house.OwnerID)
				}).Return(nil)

				resp := httptest.NewRecorder()
				h.PatchRelationship(Human{}, "houses").ServeHTTP(resp, req)

				buf := &bytes.Buffer{}
				_, err = buf.ReadFrom(resp.Body)
				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.Code)

				strings.Contains(buf.String(), `"data":null`)
			})

			t.Run("SetWithLinks", func(t *testing.T) {
				h := NewC(c)
				h.MarshalLinks = true

				req, err := http.NewRequest("PATCH", "/humen/1/relationships/houses", strings.NewReader(`{"data":[{"type":"houses", "id": "3"}]}`))
				require.NoError(t, err)

				req.Header.Add("Content-Type", jsonapi.MediaType)
				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")
				req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

				repo, err := c.GetRepository(Human{})
				require.NoError(t, err)

				humenRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				humenRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				humenRepo.On("Commit", mock.Anything, mock.Anything).Return(nil)
				humenRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					pfs := s.PrimaryFilters
					if assert.Len(t, pfs, 1) {
						pfV := pfs[0].Values
						if assert.Len(t, pfV, 1) {
							vs := pfV[0]
							assert.Equal(t, query.OpIn, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					humen, ok := s.Value.(*[]*Human)
					require.True(t, ok)

					*humen = append(*humen, &Human{ID: 1})
				}).Return(nil)

				repo, err = c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					ffs := s.ForeignFilters
					if assert.Len(t, ffs, 1) {
						ffV := ffs[0].Values
						assert.Equal(t, "owner_id", ffs[0].StructField.NeuronName())
						if assert.Len(t, ffV, 1) {
							vs := ffV[0]
							assert.Equal(t, query.OpEqual, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					houses, ok := s.Value.(*[]*House)
					require.True(t, ok)

					*houses = append(*houses, &House{ID: 3, OwnerID: 1})
				}).Return(nil)

				housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					house, ok := s.Value.(*House)
					require.True(t, ok)

					house.ID = 3
					house.OwnerID = 1
				}).Return(nil)

				// response with value
				humenRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					human, ok := s.Value.(*Human)
					require.True(t, ok)

					human.ID = 1
				}).Return(nil)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					houses, ok := s.Value.(*[]*House)
					require.True(t, ok)

					*houses = append(*houses, &House{ID: 3})
				}).Return(nil)

				resp := httptest.NewRecorder()
				h.PatchRelationship(Human{}, "houses").ServeHTTP(resp, req)

				buf := &bytes.Buffer{}
				_, err = buf.ReadFrom(resp.Body)
				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.Code)

				assert.Contains(t, buf.String(), `{"links":{"self":"/humen/1/relationships/houses","related":"/humen/1/houses"},"data":[{"type":"houses","id":"3"}]}`)
			})
			t.Run("SetWithoutLinks", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("PATCH", "/humen/1/relationships/houses", strings.NewReader(`{"data":[{"type":"houses", "id": "3"}]}`))
				require.NoError(t, err)

				req.Header.Add("Content-Type", jsonapi.MediaType)
				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")
				req = req.WithContext(context.WithValue(context.Background(), IDKey, "1"))

				repo, err := c.GetRepository(Human{})
				require.NoError(t, err)

				humenRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				humenRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				humenRepo.On("Commit", mock.Anything, mock.Anything).Return(nil)
				humenRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					pfs := s.PrimaryFilters
					if assert.Len(t, pfs, 1) {
						pfV := pfs[0].Values
						if assert.Len(t, pfV, 1) {
							vs := pfV[0]
							assert.Equal(t, query.OpIn, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					humen, ok := s.Value.(*[]*Human)
					require.True(t, ok)

					*humen = append(*humen, &Human{ID: 1})
				}).Return(nil)

				repo, err = c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("Begin", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("Commit", mock.Anything, mock.Anything).Once().Return(nil)
				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					ffs := s.ForeignFilters
					if assert.Len(t, ffs, 1) {
						ffV := ffs[0].Values
						assert.Equal(t, "owner_id", ffs[0].StructField.NeuronName())
						if assert.Len(t, ffV, 1) {
							vs := ffV[0]
							assert.Equal(t, query.OpEqual, vs.Operator)
							if assert.Len(t, vs.Values, 1) {
								assert.Equal(t, 1, vs.Values[0])
							}
						}
					}

					houses, ok := s.Value.(*[]*House)
					require.True(t, ok)

					*houses = append(*houses, &House{ID: 3, OwnerID: 1})
				}).Return(nil)

				housesRepo.On("Patch", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					house, ok := s.Value.(*House)
					require.True(t, ok)

					house.ID = 3
					house.OwnerID = 1
				}).Return(nil)

				// response with value
				humenRepo.On("Get", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					human, ok := s.Value.(*Human)
					require.True(t, ok)

					human.ID = 1
				}).Return(nil)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					houses, ok := s.Value.(*[]*House)
					require.True(t, ok)

					*houses = append(*houses, &House{ID: 3})
				}).Return(nil)

				resp := httptest.NewRecorder()
				h.PatchRelationship(Human{}, "houses").ServeHTTP(resp, req)

				buf := &bytes.Buffer{}
				_, err = buf.ReadFrom(resp.Body)
				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.Code)

				assert.Contains(t, buf.String(), `{"data":[{"type":"houses","id":"3"}]}`)
			})
		})
	})
}
