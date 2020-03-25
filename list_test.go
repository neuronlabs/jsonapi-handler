package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/neuronlabs/neuron-core"
	mocks "github.com/neuronlabs/neuron-mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/config"
	"github.com/neuronlabs/neuron-core/query"

	handlerClass "github.com/neuronlabs/jsonapi-handler/errors/class"
)

// TestHandleList tests the handleList function.
//noinspection SpellCheckingInspection
func TestHandleList(t *testing.T) {
	c, err := neuron.NewController(config.Default())
	require.NoError(t, err)

	err = c.RegisterRepository("mock", &config.Repository{DriverName: mocks.DriverName})
	require.NoError(t, err)

	err = c.RegisterModels(Human{}, House{}, Car{}, HookChecker{})
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {
		t.Run("Fieldset", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses?fields[houses]=id,address", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				assert.Len(t, s.Fieldset, 2)
				assert.Contains(t, s.Fieldset, s.Struct().Primary().NeuronName())
				addressField, ok := s.Struct().Attribute("address")
				require.True(t, ok)
				assert.Contains(t, s.Fieldset, addressField.NeuronName())

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)
				*v = append(*v, &House{ID: 1, Address: "Main Rd 52"}, &House{ID: 2, Address: "Main Rd 53"})
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.List(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				houses := make([]*House, 0)
				err := jsonapi.UnmarshalC(c, resp.Body, &houses)
				require.NoError(t, err)
				if assert.Len(t, houses, 2) {
					var is1, is2 bool
					for _, house := range houses {
						switch house.ID {
						case 1:
							is1 = true
							assert.Equal(t, "Main Rd 52", house.Address)
						case 2:
							is2 = true
							assert.Equal(t, "Main Rd 53", house.Address)
						default:
							t.Errorf("Invalid houseID: %v", house.ID)
						}
						assert.Nil(t, house.Owner)
						assert.Equal(t, 0, house.OwnerID)
					}
					assert.True(t, is1 && is2)
				}
			}
		})

		t.Run("Pagination", func(t *testing.T) {
			t.Run("WithCreator", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("GET", "/houses", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					assert.Len(t, s.Fieldset, 4)

					pagination := s.Pagination
					if assert.NotNil(t, pagination) {
						assert.Equal(t, query.PageNumberPagination, pagination.Type)
						number, size := pagination.GetNumberSize()
						assert.Equal(t, int64(1), number)
						assert.Equal(t, int64(3), size)
					}

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)
					*v = append(*v, &House{ID: 1, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 2, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

				resp := httptest.NewRecorder()
				h.ListWith(House{}).PageSize(3).Handler().ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err = jsonapi.UnmarshalC(c, resp.Body, &houses)
					require.NoError(t, err)
					if assert.Len(t, houses, 2) {
						var is1, is2 bool
						for _, house := range houses {
							switch house.ID {
							case 1:
								is1 = true
								assert.Equal(t, "Main Rd 52", house.Address)
							case 2:
								is2 = true
								assert.Equal(t, "Main Rd 53", house.Address)
							default:
								t.Errorf("Invalid houseID: %v", house.ID)
							}
							assert.NotNil(t, house.Owner)
							assert.Equal(t, 0, house.OwnerID)
						}
						assert.True(t, is1 && is2)
					}
				}
			})

			t.Run("RouterDefault", func(t *testing.T) {
				h := NewC(c)
				h.DefaultPageSize = 2

				req, err := http.NewRequest("GET", "/houses", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					assert.Len(t, s.Fieldset, 4)

					pagination := s.Pagination
					if assert.NotNil(t, pagination) {
						assert.Equal(t, query.PageNumberPagination, pagination.Type)
						number, size := pagination.GetNumberSize()
						assert.Equal(t, int64(1), number)
						assert.Equal(t, int64(2), size)
					}

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)
					*v = append(*v, &House{ID: 1, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 2, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(30), nil)

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				buf := bytes.Buffer{}
				tee := io.TeeReader(resp.Body, &buf)

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err = jsonapi.UnmarshalC(c, tee, &houses)
					require.NoError(t, err)
					if assert.Len(t, houses, 2) {
						var is1, is2 bool
						for _, house := range houses {
							switch house.ID {
							case 1:
								is1 = true
								assert.Equal(t, "Main Rd 52", house.Address)
							case 2:
								is2 = true
								assert.Equal(t, "Main Rd 53", house.Address)
							default:
								t.Errorf("Invalid houseID: %v", house.ID)
							}
							assert.NotNil(t, house.Owner)
							assert.Equal(t, 0, house.OwnerID)
						}
						assert.True(t, is1 && is2)
					}
				}

				payload := jsonapi.ManyPayload{}
				err = json.Unmarshal(buf.Bytes(), &payload)
				require.NoError(t, err)

				if assert.NotNil(t, payload.Links) {
					assert.Contains(t, payload.Links.First, "page%5Bnumber%5D=1")
					assert.Contains(t, payload.Links.First, "page%5Bsize%5D=2")
					// total 30 - last pageSize=2 pageNumber=15
					assert.Contains(t, payload.Links.Last, "page%5Bnumber%5D=15")
					assert.Contains(t, payload.Links.Last, "page%5Bsize%5D=2")

					assert.Contains(t, payload.Links.Next, "page%5Bnumber%5D=2")
					assert.Contains(t, payload.Links.Next, "page%5Bsize%5D=2")
				}

				if assert.NotNil(t, payload.Meta) {
					v, ok := (*payload.Meta)[jsonapi.KeyTotal]
					if assert.True(t, ok) {
						// json by default sets value to float64
						assert.Equal(t, float64(30), v)
					}
				}

			})

			t.Run("PageBoth", func(*testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("GET", "/houses?page[number]=1&page[size]=3", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					assert.Len(t, s.Fieldset, 4)

					pagination := s.Pagination
					if assert.NotNil(t, pagination) {
						assert.Equal(t, query.PageNumberPagination, pagination.Type)
						number, size := pagination.GetNumberSize()
						assert.Equal(t, int64(1), number)
						assert.Equal(t, int64(3), size)
					}

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)
					*v = append(*v, &House{ID: 1, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 2, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err = jsonapi.UnmarshalC(c, resp.Body, &houses)
					require.NoError(t, err)
					if assert.Len(t, houses, 2) {
						var is1, is2 bool
						for _, house := range houses {
							switch house.ID {
							case 1:
								is1 = true
								assert.Equal(t, "Main Rd 52", house.Address)
							case 2:
								is2 = true
								assert.Equal(t, "Main Rd 53", house.Address)
							default:
								t.Errorf("Invalid houseID: %v", house.ID)
							}
							assert.NotNil(t, house.Owner)
							assert.Equal(t, 0, house.OwnerID)
						}
						assert.True(t, is1 && is2)
					}
				}
			})

			t.Run("PageSize", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("GET", "/houses?page[size]=2", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					assert.Len(t, s.Fieldset, 4)

					pagination := s.Pagination
					if assert.NotNil(t, pagination) {
						assert.Equal(t, query.PageNumberPagination, pagination.Type)
						number, size := pagination.GetNumberSize()
						assert.Equal(t, int64(1), number)
						assert.Equal(t, int64(2), size)
					}

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)
					*v = append(*v, &House{ID: 1, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 2, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(52), nil)

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				buf := bytes.Buffer{}
				tee := io.TeeReader(resp.Body, &buf)

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err = jsonapi.UnmarshalC(c, tee, &houses)
					require.NoError(t, err)
					if assert.Len(t, houses, 2) {
						var is1, is2 bool
						for _, house := range houses {
							switch house.ID {
							case 1:
								is1 = true
								assert.Equal(t, "Main Rd 52", house.Address)
							case 2:
								is2 = true
								assert.Equal(t, "Main Rd 53", house.Address)
							default:
								t.Errorf("Invalid houseID: %v", house.ID)
							}
							assert.NotNil(t, house.Owner)
							assert.Equal(t, 0, house.OwnerID)
						}
						assert.True(t, is1 && is2)
					}
				}
				payload := jsonapi.ManyPayload{}
				err = json.Unmarshal(buf.Bytes(), &payload)
				require.NoError(t, err)

				t.Logf("Buf: %s", buf.Bytes())

				if assert.NotNil(t, payload.Links) {
					assert.Contains(t, payload.Links.First, "page%5Bnumber%5D=1")
					assert.Contains(t, payload.Links.First, "page%5Bsize%5D=2")
					// total 30 - last pageSize=2 pageNumber=15
					assert.Contains(t, payload.Links.Last, "page%5Bnumber%5D=26")
					assert.Contains(t, payload.Links.Last, "page%5Bsize%5D=2")

					assert.Contains(t, payload.Links.Next, "page%5Bnumber%5D=2")
					assert.Contains(t, payload.Links.Next, "page%5Bsize%5D=2")
				}

				if assert.NotNil(t, payload.Meta) {
					v, ok := (*payload.Meta)[jsonapi.KeyTotal]
					if assert.True(t, ok) {
						// json by default sets value to float64
						assert.Equal(t, float64(52), v)
					}
				}
			})

			t.Run("Offset", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("GET", "/houses?page[limit]=2&page[offset]=3", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					assert.Len(t, s.Fieldset, 4)

					require.NotNil(t, s.Pagination)

					if assert.NotNil(t, s.Pagination) {
						assert.Equal(t, query.LimitOffsetPagination, s.Pagination.Type)
						limit, offset := s.Pagination.GetLimitOffset()
						assert.Equal(t, int64(2), limit)
						assert.Equal(t, int64(3), offset)
					}

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)
					*v = append(*v, &House{ID: 4, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 5, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(10), nil)

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				buf := bytes.Buffer{}
				tee := io.TeeReader(resp.Body, &buf)

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err = jsonapi.UnmarshalC(c, tee, &houses)
					require.NoError(t, err)
					if assert.Len(t, houses, 2) {
						var is4, is5 bool
						for _, house := range houses {
							switch house.ID {
							case 4:
								is4 = true
								assert.Equal(t, "Main Rd 52", house.Address)
							case 5:
								is5 = true
								assert.Equal(t, "Main Rd 53", house.Address)
							default:
								t.Errorf("Invalid houseID: %v", house.ID)
							}
							assert.NotNil(t, house.Owner)
							assert.Equal(t, 0, house.OwnerID)
						}
						assert.True(t, is4 && is5)
					}

					payload := jsonapi.ManyPayload{}
					err = json.Unmarshal(buf.Bytes(), &payload)
					require.NoError(t, err)

					if assert.NotNil(t, payload.Links) {
						assert.NotContains(t, payload.Links.First, "page%5Boffset%5D=0")
						assert.Contains(t, payload.Links.First, "page%5Blimit%5D=2")

						assert.Contains(t, payload.Links.Last, "page%5Boffset%5D=9")
						assert.Contains(t, payload.Links.Last, "page%5Blimit%5D=2")

						assert.Contains(t, payload.Links.Next, "page%5Boffset%5D=5")
						assert.Contains(t, payload.Links.Next, "page%5Blimit%5D=2")
					}

					if assert.NotNil(t, payload.Meta) {
						v, ok := (*payload.Meta)[jsonapi.KeyTotal]
						if assert.True(t, ok) {
							// json by default sets value to float64
							assert.Equal(t, float64(10), v)
						}
					}
				}
			})
		})

		t.Run("Includes", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses?include=owner&links=false", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)
				*v = append(*v, &House{ID: 4, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 5, Address: "Main Rd 53", OwnerID: 5}, &House{ID: 6, Address: "Main Rd 54", OwnerID: 5})
			}).Return(nil)

			housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

			repo, err = c.GetRepository(Human{})
			require.NoError(t, err)

			humenRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			// get included humen
			humenRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)
				primaries := s.PrimaryFilters
				if assert.Len(t, primaries, 1) {
					pm := primaries[0]
					if assert.Len(t, pm.Values, 1) {
						pv := pm.Values[0]
						assert.Equal(t, query.OpIn, pv.Operator)
						assert.Contains(t, pv.Values, 3)
						assert.Contains(t, pv.Values, 5)
					}
				}

				v, ok := s.Value.(*[]*Human)
				require.True(t, ok)
				*v = append(*v, &Human{ID: 3, Age: 32, Name: "Sarah"}, &Human{ID: 5, Age: 55, Name: "Jessica"})
			}).Return(nil)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)
				*v = append(*v, &House{ID: 4, OwnerID: 3}, &House{ID: 5, OwnerID: 5}, &House{ID: 6, OwnerID: 5})
			}).Return(nil)

			resp := httptest.NewRecorder()
			h.List(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			if !assert.Equal(t, http.StatusOK, resp.Code) {
				data, _ := ioutil.ReadAll(resp.Body)
				t.Log(string(data))
				return
			}

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				houses := make([]*House, 0)
				buf := &bytes.Buffer{}
				tee := io.TeeReader(resp.Body, buf)

				err = jsonapi.UnmarshalC(c, tee, &houses)
				require.NoError(t, err)
				if assert.Len(t, houses, 3) {
					var is4, is5, is6 bool
					for _, house := range houses {
						switch house.ID {
						case 4:
							is4 = true
							assert.Equal(t, "Main Rd 52", house.Address)
						case 5:
							is5 = true
							assert.Equal(t, "Main Rd 53", house.Address)
						case 6:
							is6 = true
							assert.Equal(t, "Main Rd 54", house.Address)
						default:
							t.Errorf("Invalid houseID: %v", house.ID)
						}
						assert.NotNil(t, house.Owner)
						assert.Equal(t, 0, house.OwnerID)
					}
					assert.True(t, is4 && is5 && is6)
				}

				payload := jsonapi.ManyPayload{}
				err = json.Unmarshal(buf.Bytes(), &payload)
				require.NoError(t, err)

				var has3, has5 bool
				if assert.Len(t, payload.Included, 2) {
					for _, included := range payload.Included {
						switch included.ID {
						case "3":
							has3 = true
						case "5":
							has5 = true
						}
						assert.Equal(t, "humen", included.Type)
					}
				}
				assert.True(t, has3 && has5)
			}
		})

		t.Run("Filters", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses?filter[houses][id][$ge]=30&lang=pl", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				assert.Len(t, s.Fieldset, 4)

				primaries := s.PrimaryFilters
				if assert.Len(t, primaries, 1) {
					first := primaries[0]
					if assert.Len(t, first.Values, 1) {
						fv := first.Values[0]
						assert.Equal(t, query.OpGreaterEqual, fv.Operator)

						if assert.Len(t, fv.Values, 1) {
							assert.Equal(t, "30", fv.Values[0])
						}
					}
				}
				v, ok := s.Value.(*[]*House)
				require.True(t, ok)

				*v = append(*v, &House{ID: 31, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 32, Address: "Main Rd 53", OwnerID: 5})
			}).Return(nil)

			housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

			resp := httptest.NewRecorder()
			h.List(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			if !assert.Equal(t, http.StatusOK, resp.Code) {
				data, _ := ioutil.ReadAll(resp.Body)
				t.Log(string(data))
				return
			}

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				houses := make([]*House, 0)
				err = jsonapi.UnmarshalC(c, resp.Body, &houses)
				require.NoError(t, err)
				if assert.Len(t, houses, 2) {
					var is31, is32 bool
					for _, house := range houses {
						switch house.ID {
						case 31:
							is31 = true
							assert.Equal(t, "Main Rd 52", house.Address)
						case 32:
							is32 = true
							assert.Equal(t, "Main Rd 53", house.Address)
						default:
							t.Errorf("Invalid houseID: %v", house.ID)
						}
						assert.NotNil(t, house.Owner)
						assert.Equal(t, 0, house.OwnerID)
					}
					assert.True(t, is31 && is32)
				}
			}
		})

		t.Run("Sorts", func(t *testing.T) {
			t.Run("Endpoint", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("GET", "/houses", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)

					if sorts := s.SortFields; assert.Len(t, sorts, 1) {
						sf := sorts[0]
						assert.Equal(t, s.Struct().Primary(), sf.StructField)
						assert.Equal(t, query.DescendingOrder, sf.Order)
					}

					*v = append(*v, &House{ID: 5, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 4, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

				resp := httptest.NewRecorder()
				h.ListWith(House{}).SortOrder("-id").Handler().ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err := jsonapi.UnmarshalC(c, resp.Body, &houses)
					require.NoError(t, err)
					assert.Len(t, houses, 2)

					assert.Equal(t, 5, houses[0].ID)
					assert.Equal(t, 4, houses[1].ID)
				}
			})

			t.Run("Query", func(t *testing.T) {
				h := NewC(c)

				req, err := http.NewRequest("GET", "/houses?sort=-id", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				repo, err := c.GetRepository(House{})
				require.NoError(t, err)

				housesRepo, ok := repo.(*mocks.Repository)
				require.True(t, ok)

				housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
					s, ok := args[1].(*query.Scope)
					require.True(t, ok)

					v, ok := s.Value.(*[]*House)
					require.True(t, ok)

					if sorts := s.SortFields; assert.Len(t, sorts, 1) {
						sf := sorts[0]
						assert.Equal(t, s.Struct().Primary(), sf.StructField)
						assert.Equal(t, query.DescendingOrder, sf.Order)
					}

					*v = append(*v, &House{ID: 5, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 4, Address: "Main Rd 53", OwnerID: 5})
				}).Return(nil)

				housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusOK, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					houses := make([]*House, 0)
					err = jsonapi.UnmarshalC(c, resp.Body, &houses)
					require.NoError(t, err)
					assert.Len(t, houses, 2)

					assert.Equal(t, 5, houses[0].ID)
					assert.Equal(t, 4, houses[1].ID)
				}
			})
		})

		t.Run("Links", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses?links=false", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)
				*v = append(*v, &House{ID: 4, Address: "Main Rd 52", OwnerID: 3}, &House{ID: 5, Address: "Main Rd 53", OwnerID: 5})
			}).Return(nil)

			housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

			resp := httptest.NewRecorder()
			h.List(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			if !assert.Equal(t, http.StatusOK, resp.Code) {
				data, _ := ioutil.ReadAll(resp.Body)
				t.Log(string(data))
				return
			}

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				buf := &bytes.Buffer{}
				tee := io.TeeReader(resp.Body, buf)
				houses := make([]*House, 0)
				err = jsonapi.UnmarshalC(c, tee, &houses)
				require.NoError(t, err)
				assert.Len(t, houses, 2)

				assert.NotContains(t, buf.String(), "links")
			}
		})

		t.Run("TotalPage", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses?fields[houses]&page[total]", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
				s, ok := args[1].(*query.Scope)
				require.True(t, ok)

				assert.Len(t, s.Fieldset, 1)
				assert.Contains(t, s.Fieldset, s.Struct().Primary().NeuronName())

				v, ok := s.Value.(*[]*House)
				require.True(t, ok)
				*v = append(*v, &House{ID: 1}, &House{ID: 2})
			}).Return(nil)

			housesRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

			resp := httptest.NewRecorder()
			h.List(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			require.Equal(t, http.StatusOK, resp.Code)

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				houses := make([]*House, 0)
				err = jsonapi.UnmarshalC(c, resp.Body, &houses)
				require.NoError(t, err)
				if assert.Len(t, houses, 2) {
					var is1, is2 bool
					for _, house := range houses {
						switch house.ID {
						case 1:
							is1 = true
						case 2:
							is2 = true
						default:
							t.Errorf("Invalid houseID: %v", house.ID)
						}
						assert.Nil(t, house.Owner)
						assert.Equal(t, 0, house.OwnerID)
					}
					assert.True(t, is1 && is2)
				}
			}
		})
	})

	t.Run("NotFound", func(t *testing.T) {
		h := NewC(c)

		req, err := http.NewRequest("GET", "/houses", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")

		repo, err := c.GetRepository(House{})
		require.NoError(t, err)

		housesRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		housesRepo.On("List", mock.Anything, mock.Anything).Once().Return(errors.New(class.QueryValueNoResult, "not found"))

		resp := httptest.NewRecorder()
		h.List(House{}).ServeHTTP(resp, req)

		// the status should be 200.
		if !assert.Equal(t, http.StatusOK, resp.Code) {
			data, _ := ioutil.ReadAll(resp.Body)
			t.Log(string(data))
			return
		}

		if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
			houses := make([]*House, 0)
			err = jsonapi.UnmarshalC(c, resp.Body, &houses)
			require.NoError(t, err)
			assert.Len(t, houses, 0)
		}
	})

	t.Run("Error", func(t *testing.T) {
		t.Run("Repository", func(t *testing.T) {
			h := NewC(c)

			req, err := http.NewRequest("GET", "/houses", nil)
			require.NoError(t, err)

			req.Header.Add("Accept", jsonapi.MediaType)
			req.Header.Add("Accept-Encoding", "identity")

			repo, err := c.GetRepository(House{})
			require.NoError(t, err)

			housesRepo, ok := repo.(*mocks.Repository)
			require.True(t, ok)

			housesRepo.On("List", mock.Anything, mock.Anything).Once().Return(errors.New(class.RepositoryConnectionTimedOut, "no connection"))

			resp := httptest.NewRecorder()
			h.List(House{}).ServeHTTP(resp, req)

			// the status should be 200.
			if !assert.Equal(t, http.StatusServiceUnavailable, resp.Code) {
				data, _ := ioutil.ReadAll(resp.Body)
				t.Log(string(data))
				return
			}

			if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
				payload, err := jsonapi.UnmarshalErrors(resp.Body)
				require.NoError(t, err)

				if assert.Len(t, payload.Errors, 1) {
					e := payload.Errors[0]
					code, err := strconv.ParseInt(e.Code, 16, 32)
					require.NoError(t, err)

					assert.Equal(t, class.RepositoryConnectionTimedOut, errors.Class(code))
				}
			}
		})

		t.Run("Query", func(t *testing.T) {
			t.Run("Single", func(t *testing.T) {
				h := NewC(c)
				h.StrictQueriesMode = true

				req, err := http.NewRequest("GET", "/houses?invalid", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					payload, err := jsonapi.UnmarshalErrors(resp.Body)
					require.NoError(t, err)

					if assert.Len(t, payload.Errors, 1) {
						e := payload.Errors[0]
						code, err := strconv.ParseInt(e.Code, 16, 32)
						require.NoError(t, err)

						assert.Equal(t, handlerClass.QueryInvalidParameter, errors.Class(code))
					}
				}
			})

			t.Run("Multiple", func(t *testing.T) {
				h := NewC(c)
				h.StrictQueriesMode = true
				h.QueryErrorsLimit = 4

				req, err := http.NewRequest("GET", "/houses?fields[some][invalid]=format&filters[humen][id][$eq]=3&sort=nosuchfield&fields[other][invalid]=invalid&sort=invalidField&fields[another][invalid]", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}

				if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
					payload, err := jsonapi.UnmarshalErrors(resp.Body)
					require.NoError(t, err)

					if assert.Len(t, payload.Errors, 4) {
						for _, e := range payload.Errors {
							code, err := strconv.ParseInt(e.Code, 16, 32)
							require.NoError(t, err)

							switch errors.Class(code) {
							case handlerClass.QueryInvalidParameter:
							case class.QueryFieldsetInvalid:
							case class.QuerySortField:
							case class.QueryFilterUnknownCollection:
							default:
								t.Errorf("Invalid error code: %32b", code)
							}
						}
					}
				}
			})

			t.Run("ManyValues", func(t *testing.T) {
				h := NewC(c)
				h.StrictQueriesMode = true
				h.QueryErrorsLimit = 4

				req, err := http.NewRequest("GET", "/houses?invalid&invalid=key", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}
			})

			t.Run("SortInvalid", func(t *testing.T) {
				h := NewC(c)
				h.StrictQueriesMode = true
				h.QueryErrorsLimit = 4

				req, err := http.NewRequest("GET", "/houses?sort=nosuchfield", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}
			})

			t.Run("FilterInvalid", func(t *testing.T) {
				h := NewC(c)
				h.StrictQueriesMode = true
				h.QueryErrorsLimit = 4

				req, err := http.NewRequest("GET", "/houses?filter[too][long][filter][field][$id]=3", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}
			})

			t.Run("FieldsetInvalid", func(t *testing.T) {
				h := NewC(c)
				h.StrictQueriesMode = true
				h.QueryErrorsLimit = 4

				req, err := http.NewRequest("GET", "/houses?fields[anything][else]=invalid&fields[unknown]&fields[humen]=age", nil)
				require.NoError(t, err)

				req.Header.Add("Accept", jsonapi.MediaType)
				req.Header.Add("Accept-Encoding", "identity")

				resp := httptest.NewRecorder()
				h.List(House{}).ServeHTTP(resp, req)

				// the status should be 200.
				if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
					data, _ := ioutil.ReadAll(resp.Body)
					t.Log(string(data))
					return
				}
			})

			t.Run("InvalidPagination", func(t *testing.T) {
				t.Run("Value", func(t *testing.T) {
					h := NewC(c)
					h.StrictQueriesMode = true
					h.QueryErrorsLimit = 4

					req, err := http.NewRequest("GET", "/houses?page[number]=invalid", nil)
					require.NoError(t, err)

					req.Header.Add("Accept", jsonapi.MediaType)
					req.Header.Add("Accept-Encoding", "identity")

					resp := httptest.NewRecorder()
					h.List(House{}).ServeHTTP(resp, req)

					// the status should be 200.
					if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
						data, _ := ioutil.ReadAll(resp.Body)
						t.Log(string(data))
						return
					}
				})

				t.Run("MultipleTypes", func(t *testing.T) {
					h := NewC(c)
					h.StrictQueriesMode = true
					h.QueryErrorsLimit = 4

					req, err := http.NewRequest("GET", "/houses?page[number]=3&page[limit]=3", nil)
					require.NoError(t, err)

					req.Header.Add("Accept", jsonapi.MediaType)
					req.Header.Add("Accept-Encoding", "identity")

					resp := httptest.NewRecorder()
					h.List(House{}).ServeHTTP(resp, req)

					// the status should be 200.
					if !assert.Equal(t, http.StatusBadRequest, resp.Code) {
						data, _ := ioutil.ReadAll(resp.Body)
						t.Log(string(data))
						return
					}
				})
			})
		})
	})

	t.Run("Hooks", func(t *testing.T) {
		h := NewC(c)
		RegisterHookC(c, HookChecker{}, BeforeList, hooksCheckerBeforeList)
		RegisterHookC(c, HookChecker{}, AfterList, hooksCheckerAfterList)

		req, err := http.NewRequest("GET", "/hook_checkers?fields[hook_checkers]=after", nil)
		require.NoError(t, err)

		req.Header.Add("Accept", jsonapi.MediaType)
		req.Header.Add("Accept-Encoding", "identity")

		repo, err := c.GetRepository(HookChecker{})
		require.NoError(t, err)

		hookCheckerRepo, ok := repo.(*mocks.Repository)
		require.True(t, ok)

		hookCheckerRepo.On("List", mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {
			s, ok := args[1].(*query.Scope)
			require.True(t, ok)

			v, ok := s.Value.(*[]*HookChecker)
			require.True(t, ok)

			// hook before
			_, ok = s.Fieldset["before"]
			assert.True(t, ok)

			*v = append(*v, &HookChecker{ID: 1, Before: true}, &HookChecker{ID: 2, Before: false})
		}).Return(nil)
		// return count
		hookCheckerRepo.On("Count", mock.Anything, mock.Anything).Once().Return(int64(2), nil)

		resp := httptest.NewRecorder()
		h.List(HookChecker{}).ServeHTTP(resp, req)

		// the status should be 200.
		require.Equal(t, http.StatusOK, resp.Code)

		if assert.Equal(t, jsonapi.MediaType, resp.Header().Get("Content-Type")) {
			hcs := make([]*HookChecker, 0)
			err = jsonapi.UnmarshalC(c, resp.Body, &hcs)
			require.NoError(t, err)
			if assert.Len(t, hcs, 2) {
				var is1, is2 bool
				for _, hc := range hcs {
					switch hc.ID {
					case 1:
						is1 = true
						assert.True(t, hc.Before)
						assert.Equal(t, 0, hc.Number)
					case 2:
						is2 = true
						assert.False(t, hc.Before)
						assert.Equal(t, 1, hc.Number)
					default:
						t.Errorf("Invalid houseID: %v", hc.ID)
					}
				}
				assert.True(t, is1 && is2)
			}
		}
	})
}
