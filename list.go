package handler

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/annotation"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	handlerErrors "github.com/neuronlabs/jsonapi-handler/errors"
	handlerClass "github.com/neuronlabs/jsonapi-handler/errors/class"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// ListHandlerCreator is the creator for the JSONAPI list endpoint http.Handler.
type ListHandlerCreator struct {
	h          *Creator
	model      *mapping.ModelStruct
	basePath   string
	pageSize   int
	sortFields []string
}

// BasePath sets the basePath for given endpoint.
func (l *ListHandlerCreator) BasePath(basePath string) *ListHandlerCreator {
	l.basePath = basePath
	return l
}

// Handler returns http.HandlerFunc for given handler creator.
func (l *ListHandlerCreator) Handler() http.HandlerFunc {
	return l.h.handleList(l.model, l.pageSize, l.basePath, l.sortFields...)
}

// PageSize sets the default 'pageSize' for given endpoint.
func (l *ListHandlerCreator) PageSize(pageSize int) *ListHandlerCreator {
	l.pageSize = pageSize
	return l
}

// SortOrder sets the default sorting order for given endpoint. The input values should be in format:
// field1, -field2 		- order by ascending field1 and then by descending field2.
func (l *ListHandlerCreator) SortOrder(defaultSortOrder ...string) *ListHandlerCreator {
	l.sortFields = defaultSortOrder
	return l
}

// ListWith returns JSONAPI list endpoint http.Handler Creator for given 'model'.
func (h *Creator) ListWith(model interface{}) *ListHandlerCreator {
	return &ListHandlerCreator{
		h:     h,
		model: h.c.MustGetModelStruct(model),
	}
}

// List returns JSONAPI list http.HandlerFunc for given 'model'.
func (h *Creator) List(model interface{}) http.HandlerFunc {
	return h.handleList(h.c.MustGetModelStruct(model), 0, "")
}

func (h *Creator) handleList(model *mapping.ModelStruct, defaultPageSize int, basePath string, defaultSortOrder ...string) http.HandlerFunc {
	var defaultPagination *query.Pagination
	if defaultPageSize <= 0 && h.DefaultPageSize > 0 {
		defaultPageSize = h.DefaultPageSize
	}
	if defaultPageSize > 0 {
		defaultPagination = &query.Pagination{
			Size:   int64(defaultPageSize),
			Offset: 1,
			Type:   query.PageNumberPagination,
		}
		log.Debug2f("Default pagination at 'GET /%s' is: %v", model.Collection(), defaultPagination.String())
	}
	var defaultSortOrderFields []*query.SortField
	// get default sorting from the endpoint config.
	if len(defaultSortOrder) > 0 {
		var err error
		defaultSortOrderFields, err = query.NewSortFields(model, true, defaultSortOrder...)
		if err != nil {
			log.Panicf("sorting order for the model: '%s' failed: '%v'", model.String(), err)
		}
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		s, err := h.createListScope(ctx, model, req)
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}

		if defaultPagination != nil && s.Pagination == nil {
			// TODO: add possibility to set nil pagination
			s.Pagination = &query.Pagination{}
			*s.Pagination = *defaultPagination
		}

		if defaultSortOrderFields != nil {
			if len(s.SortFields) == 0 {
				for _, sortField := range defaultSortOrderFields {
					if err := s.SortField(sortField); err != nil {
						log.Errorf("[LIST][SCOPE][%s] Appending default sort field failed: %v", s.ID(), err)
						h.marshalErrors(rw, req, 0, handlerErrors.ErrInternalError())
						return
					}
				}
			}
		}

		if log.Level() >= log.LDEBUG3 {
			log.Debug3f("[LIST] %s", s.String())
		}

		// execute hook before list
		if beforeListHook, ok := Hooks.getHook(model, BeforeList); ok {
			if err = beforeListHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
				return
			}
		}

		var isNoResult bool
		if err := s.ListContext(ctx); err != nil {
			if e, ok := err.(errors.ClassError); ok {
				if e.Class() == class.QueryValueNoResult {
					isNoResult = true
				}
			}
			if !isNoResult {
				h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
				return
			}
		}
		// execute the after list hook if given model implements it.
		if afterListHook, ok := Hooks.getHook(model, AfterList); ok {
			if err = afterListHook(ctx, s); err != nil {
				h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
				return
			}
		}

		linkType := jsonapi.ResourceLink
		if !h.MarshalLinks {
			linkType = jsonapi.NoLink
		}
		options := &jsonapi.MarshalOptions{Link: jsonapi.LinkOptions{
			Type:       linkType,
			BaseURL:    h.getBasePath(basePath),
			Collection: model.Collection(),
		}}

		// if there were a query no set link type to 'NoLink'
		if v, ok := s.StoreGet(scopeLinksK); ok {
			if !v.(bool) {
				options.Link.Type = jsonapi.NoLink
			}
		}

		// if there is no pagination then the pagination doesn't need to be created.
		// marshal the results if there were no pagination set
		if s.Pagination == nil || isNoResult {
			h.marshalScope(s, rw, req, http.StatusOK, options)
			return
		}

		// prepare new count scope - and build query parameters for the pagination
		// page[limit] page[offset] page[number] page[size]
		countScope := s.Copy()
		total, err := countScope.CountContext(ctx)
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}

		temp := h.queryWithoutPagination(req)

		// extract query values from the req.URL
		// prepare the pagination links for the options
		if s.Pagination != nil {
			s.Pagination.FormatQuery(temp)
		}
		paginationLinks := &jsonapi.PaginationLinks{Total: total}
		options.Link.PaginationLinks = paginationLinks
		options.Link.PaginationLinks.Self = temp.Encode()

		next, err := s.Pagination.Next(total)
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}
		temp = h.queryWithoutPagination(req)

		if next != s.Pagination {
			next.FormatQuery(temp)
			paginationLinks.Next = temp.Encode()
			temp = url.Values{}
		}

		prev, err := s.Pagination.Previous()
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}
		if prev != s.Pagination {
			prev.FormatQuery(temp)
			paginationLinks.Prev = temp.Encode()
			temp = h.queryWithoutPagination(req)
		}

		last, err := s.Pagination.Last(total)
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}
		last.FormatQuery(temp)

		paginationLinks.Last = temp.Encode()

		temp = h.queryWithoutPagination(req)
		first, err := s.Pagination.First()
		if err != nil {
			h.marshalErrors(rw, req, 0, handlerErrors.MapError(err)...)
			return
		}
		first.FormatQuery(temp)
		paginationLinks.First = temp.Encode()
		h.marshalScope(s, rw, req, http.StatusOK, options)
	}
}

func (h *Creator) queryWithoutPagination(req *http.Request) url.Values {
	temp := url.Values{}

	for k, v := range req.URL.Query() {
		switch k {
		case query.ParamPageLimit, query.ParamPageNumber, query.ParamPageOffset, query.ParamPageSize:
		default:
			temp[k] = v
		}
	}
	return temp
}

const (
	// QueryParamPageTotal is the query parameter that marks to get total number instances / pages for the collection.
	QueryParamPageTotal string = "page[total]"
	// QueryParamLinks is the query parameter that marks to marshal object links.
	QueryParamLinks string = "links"
)

func (h *Creator) createListScope(ctx context.Context, model *mapping.ModelStruct, req *http.Request) (*query.Scope, error) {
	var multiErrors errors.MultiError
	s := query.NewModelC(h.c, model, true)
	q := req.URL.Query()

	// Included
	included, ok := q[query.ParamInclude]
	if ok {
		includedFields := strings.Split(included[0], annotation.Separator)
		err := s.IncludeFields(includedFields...)
		if err != nil {
			return nil, err
		}
	}

	languages, ok := q[query.ParamLanguage]
	if ok {
		err := h.queryParameterLanguage(s, languages[0])
		if err != nil {
			return nil, err
		}
	}

	for key, values := range q {
		if len(multiErrors) >= h.QueryErrorsLimit && h.QueryErrorsLimit != 0 {
			log.Debug2f("Reached single query error limit: %v", multiErrors)
			return nil, multiErrors
		}

		if len(values) > 1 {
			err := errors.NewDetf(handlerClass.QueryInvalidParameter, "provided invalid query parameters")
			err.SetDetailsf("The query parameter: '%s' used more than once.", key)
			multiErrors = append(multiErrors, err)
			continue
		}

		value := values[0]
		select {
		case <-ctx.Done():
			ctxErr := ctx.Err()
			if ctxErr == context.DeadlineExceeded {
				err := errors.NewDet(handlerClass.QueryTimeout, context.DeadlineExceeded.Error())
				err.SetDetails("The query connection had timed out")
				return nil, err
			}
			return nil, ctxErr
		default:
		}

		var err error
		switch {
		case key == query.ParamInclude, key == query.ParamLanguage:
			continue
		case key == query.ParamPageLimit:
			err = preparePagination(s, key, value, ppLimit)
		case key == query.ParamPageOffset:
			err = preparePagination(s, key, value, ppOffset)
		case key == query.ParamPageNumber:
			err = preparePagination(s, key, value, ppPageNumber)
		case key == query.ParamPageSize:
			err = preparePagination(s, key, value, ppPageSize)
		case key == query.ParamSort:
			err = h.queryParameterSort(s, value)
		case strings.HasPrefix(key, query.ParamFilter):
			err = h.queryParameterFilters(s, key, value)
		case strings.HasPrefix(key, query.ParamFields):
			err = h.queryParameterFields(s, key, value)
		case key == QueryParamPageTotal:
			err = h.queryParameterPageTotal(s, key, value)
		case key == QueryParamLinks:
			err = h.queryParameterLinks(s, key, value)
		default:
			err = h.defaultQueryParameter(key)
		}

		if err != nil {
			if ce, ok := err.(errors.ClassError); ok {
				multiErrors = append(multiErrors, ce)
			} else if me, ok := err.(errors.MultiError); ok {
				multiErrors = append(multiErrors, me...)
			} else {
				return nil, err
			}
		}
	}
	if len(multiErrors) > 0 {
		log.Debug2f("Multiple errors: %v", multiErrors)
		return nil, multiErrors
	}

	if s.Pagination != nil {
		if s.Pagination.Type == query.PageNumberPagination && s.Pagination.Offset == 0 {
			if _, ok := q[query.ParamPageNumber]; !ok {
				// if the default pageNumber is not set within the query, but the pagination
				// type is PageNumberPagination - set it's pageNumber to default - 1.
				s.Pagination.Offset = 1
			}
		}
		if err := s.Pagination.IsValid(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (h *Creator) queryParameterLanguage(s *query.Scope, value string) error {
	languageField := s.Struct().LanguageField()
	if languageField == nil {
		// return no error if no language field is found.
		return nil
	}

	err := s.FilterField(query.NewFilter(s.Struct().LanguageField(), query.OpIn, value))
	if err != nil {
		return err
	}
	return nil
}

func (h *Creator) queryParameterFields(s *query.Scope, key, value string) error {
	if log.Level() == log.LDEBUG3 {
		log.Debug3f("query fields parameter: '%s' - '%s'", key, value)
	}

	split, err := query.SplitBracketParameter(key[len(query.ParamFields):])
	if err != nil {
		return err
	}

	if len(split) != 1 {
		err := errors.NewDetf(handlerClass.QueryInvalidParameter, "invalid fields parameter")
		err.SetDetailsf("The fields parameter has invalid form. %s", key)
		return err
	}

	model, err := h.c.ModelStruct(split[0])
	if err != nil {
		if log.Level() == log.LDEBUG3 {
			log.Debug3f("[QUERY][%s] invalid fieldset model: '%s': %v", s.ID(), split[0], err.Error())
		}
		err := errors.NewDetf(handlerClass.QueryInvalidParameter, "invalid query parameter")
		err.SetDetailsf("Fields query parameter contains invalid collection name: '%s'", split[0])
		return err
	}

	fieldsScope := s
	if s.Struct() != model {
		fieldsScope, err = s.IncludedScope(model)
		if err != nil {
			return err
		}
	}

	var (
		fields    []interface{}
		isPrimary bool
	)

	if value != "" {
		for _, field := range strings.Split(value, annotation.Separator) {
			if field == "id" || field == "-id" {
				isPrimary = true
			}
			fields = append(fields, field)
		}
	}

	if !isPrimary {
		fields = append(fields, "id")
	}

	if log.Level() == log.LDEBUG3 {
		log.Debug3f("Setting fieldset to: %v", fields)
	}

	return fieldsScope.SetFieldset(fields...)
}

func (h *Creator) queryParameterFilters(s *query.Scope, key, value string) error {
	f, err := query.NewStringFilter(h.c, key, value)
	if err != nil {
		return err
	}

	filterScope := s
	if filterModel := f.StructField.ModelStruct(); filterModel != s.Struct() {
		filterScope, err = s.IncludedScope(filterModel)
		if err != nil {
			err := errors.NewDetf(class.QueryFilterUnknownCollection, "invalid query collection: '%s'", filterModel.Collection())
			err.SetDetailsf("Filtering collection: '%s' is not included", filterModel.Collection())
			return err
		}
	}
	return filterScope.FilterField(f)
}

func (h *Creator) queryParameterSort(s *query.Scope, value string) error {
	fields := strings.Split(value, annotation.Separator)
	return s.Sort(fields...)
}

func (h *Creator) queryParameterPageTotal(s *query.Scope, key, value string) error {
	countTotal := true
	var err error
	if value != "" {
		countTotal, err = strconv.ParseBool(value)
		if err != nil {
			err := errors.NewDetf(handlerClass.QueryInvalidParameter, "invalid query parameter: '%s' value: '%s'", key, value)
			err.SetDetailsf("Query parameter: '%s' allows empty or boolean values only.", key)
			return err
		}
	}
	s.StoreSet(scopeCountListK, countTotal)
	return nil
}

type paginationParam int

const (
	ppLimit paginationParam = iota
	ppOffset
	ppPageNumber
	ppPageSize
)

func (h *Creator) queryParameterLinks(s *query.Scope, key, value string) error {
	useLinks := true
	var err error
	if value != "" {
		useLinks, err = strconv.ParseBool(value)
		if err != nil {
			err := errors.NewDetf(handlerClass.QueryInvalidParameter, "invalid query links parameter: '%s'", err.Error())
			err.SetDetailsf("Query parameter: '%s' contains non boolean value: '%s'.", key, value)
			return err
		}
	}
	s.StoreSet(scopeLinksK, useLinks)
	return nil
}

func (h *Creator) defaultQueryParameter(key string) error {
	if !h.StrictQueriesMode {
		return nil
	}
	err := errors.NewDetf(handlerClass.QueryInvalidParameter, "unsupported query parameter: '%s'", key)
	err.SetDetailsf("Query parameter: '%s' is not supported by the server.", key)
	return err
}

// preparePagination prepares the pagination for provided 's' query scope.
func preparePagination(s *query.Scope, key, value string, index paginationParam) error {
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		err := errors.NewDet(class.QueryPaginationValue, "invalid pagination value")
		err.SetDetailsf("Provided query parameter: %v, contains invalid value: %v. Positive integer value is required.", key, value)
		return err
	}

	switch index {
	case ppLimit, ppOffset:
		if s.Pagination == nil {
			s.Pagination = &query.Pagination{Type: query.LimitOffsetPagination}
		} else if s.Pagination.Type == query.PageNumberPagination {
			log.Debug2f("Multiple paginations type for the query")
			err := errors.NewDet(class.QueryPaginationType, "multiple pagination types")
			err.SetDetails("Multiple pagination types in the query")
			return err
		}
		if index == ppLimit {
			s.Pagination.Size = val
		} else {
			s.Pagination.Offset = val
		}
	case ppPageNumber, ppPageSize:
		if s.Pagination == nil {
			s.Pagination = &query.Pagination{Type: query.PageNumberPagination}
		} else if s.Pagination.Type == query.LimitOffsetPagination {
			log.Debug2("Multiple pagination type for the query")
			err := errors.NewDet(class.QueryPaginationType, "multiple pagination types")
			err.SetDetails("Multiple pagination types in the query")
			return err
		}
		if index == ppPageNumber {
			s.Pagination.Offset = val
		} else {
			s.Pagination.Size = val
		}
	}
	return nil
}

var scopeCountListK scopeCountList

type scopeCountList struct{}

var scopeLinksK scopeLinks

type scopeLinks struct{}
