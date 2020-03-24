package errors

import (
	"strconv"

	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"

	"github.com/neuronlabs/jsonapi-handler/log"
)

// DefaultClassMapper is the default error classification mapper.
var DefaultClassMapper = &ClassMapper{
	Majors: map[errors.Major]Creator{
		class.MjrCommon:     ErrInternalError,
		class.MjrConfig:     ErrInternalError,
		class.MjrEncoding:   ErrInvalidInput,
		class.MjrLanguage:   ErrLanguageNotAcceptable,
		class.MjrInternal:   ErrInternalError,
		class.MjrQuery:      ErrInvalidQueryParameter,
		class.MjrRepository: ErrServiceUnavailable,
		class.MjrModel:      ErrInternalError,
	},
	Minors: map[errors.Minor]Creator{
		class.MnrEncodingUnmarshal:       ErrInvalidInput,
		class.MnrEncodingMarshal:         ErrInternalError,
		class.MnrQueryFieldset:           ErrInvalidQueryParameter,
		class.MnrQuerySelectedFields:     ErrInternalError,
		class.MnrQueryPagination:         ErrInvalidQueryParameter,
		class.MnrQueryFilter:             ErrInvalidQueryParameter,
		class.MnrQuerySorts:              ErrInvalidQueryParameter,
		class.MnrQueryViolation:          ErrInvalidJSONFieldValue,
		class.MnrQueryInclude:            ErrInvalidQueryParameter,
		class.MnrQueryValue:              ErrInvalidInput,
		class.MnrQueryTransaction:        ErrInternalError,
		class.MnrRepositoryNotImplements: ErrInternalError,
		class.MnrRepositoryConnection:    ErrServiceUnavailable,
		class.MnrRepositoryReplica:       ErrInternalError,
	},
	Class: map[errors.Class]Creator{
		class.EncodingUnmarshalCollection:      ErrTypeConflict,
		class.EncodingUnmarshalUnknownField:    ErrInvalidResourceName,
		class.EncodingUnmarshalInvalidID:       ErrInvalidJSONFieldValue,
		class.EncodingUnmarshalInvalidFormat:   ErrInvalidJSONDocument,
		class.EncodingUnmarshalInvalidTime:     ErrInvalidJSONFieldValue,
		class.EncodingUnmarshalInvalidType:     ErrInvalidJSONFieldValue,
		class.EncodingUnmarshalValueOutOfRange: ErrInputOutOfRange,
		class.EncodingUnmarshalNoData:          ErrInvalidInput,

		class.QueryFieldsetDuplicate:    ErrInvalidQueryParameter,
		class.QueryFieldsetTooBig:       ErrInvalidQueryParameter,
		class.QueryFieldsetUnknownField: ErrInvalidResourceName,
		class.QueryFieldsetInvalid:      ErrInternalError,

		class.QueryFilterUnknownCollection:   ErrInvalidResourceName,
		class.QueryFilterUnknownField:        ErrInvalidResourceName,
		class.QueryFilterUnknownOperator:     ErrInvalidQueryParameter,
		class.QueryFilterMissingRequired:     ErrMissingRequiredQueryParameter,
		class.QueryFilterUnsupportedField:    ErrUnsupportedField,
		class.QueryFilterUnsupportedOperator: ErrUnsupportedFilterOperator,
		class.QueryFilterLanguage:            ErrLanguageNotAcceptable,
		class.QueryFilterInvalidField:        ErrInvalidQueryParameter,
		class.QueryFilterInvalidFormat:       ErrInvalidQueryParameter,

		class.QuerySortField:         ErrInvalidResourceName,
		class.QuerySortFormat:        ErrInvalidQueryParameter,
		class.QuerySortRelatedFields: ErrUnsupportedQueryParameter,
		class.QuerySortTooManyFields: ErrInvalidQueryParameter,

		class.QueryPaginationAlreadySet: ErrInternalError,

		class.QueryViolationCheck:       ErrInvalidJSONFieldValue,
		class.QueryViolationUnique:      ErrResourceAlreadyExists,
		class.QueryViolationNotNull:     ErrInvalidJSONFieldValue,
		class.QueryValueValidation:      ErrInvalidJSONFieldValue,
		class.QueryValueUnaddressable:   ErrInternalError,
		class.QueryValueType:            ErrInternalError,
		class.QueryValuePrimary:         ErrInvalidJSONFieldValue,
		class.QueryValueMissingRequired: ErrInvalidInput,
		class.QueryValueNoResult:        ErrResourceNotFound,

		class.QueryIncludeTooMany: ErrInvalidQueryParameter,

		class.RepositoryNotFound:       ErrInternalError,
		class.RepositoryAuthPrivileges: ErrInternalError,

		class.CommonParseBrackets: ErrInvalidQueryParameter,
		class.ModelSchemaNotFound: ErrInternalError,
	},
}

// MapError maps the 'err' input error into slice of 'Error'.
// The function uses DefaultClassMapper for error mapping.
// The logic is the same as for DefaultClassMapper.Errors method.
func MapError(err error) []*jsonapi.Error {
	return DefaultClassMapper.errors(err)
}

// ClassMapper is the neuron errors classification mapper.
// It creates the 'Error' from the provided error.
type ClassMapper struct {
	Majors map[errors.Major]Creator
	Minors map[errors.Minor]Creator
	Class  map[errors.Class]Creator
}

// Errors gets the slice of 'Error' from the provided 'err' error.
// The mapping is based on the 'most specific classification first' method.
// If the error is 'errors.ClassError' the function gets it's class.
// The function checks classification occurrence based on the priority:
//	- Class
//	- Minor
//	- Major
// If no mapping is provided for given classification - an internal error is returned.
func (c *ClassMapper) Errors(err error) []*jsonapi.Error {
	return c.errors(err)
}

func (c *ClassMapper) errors(err error) []*jsonapi.Error {
	switch et := err.(type) {
	case errors.ClassError:
		return []*jsonapi.Error{c.mapSingleError(et)}
	case errors.MultiError:
		var errs []*jsonapi.Error
		for _, single := range et {
			errs = append(errs, c.mapSingleError(single))
		}
		return errs
	default:
		log.Debugf("Unknown error: %+v", err)
	}
	return []*jsonapi.Error{ErrInternalError()}
}

func (c *ClassMapper) mapSingleError(e errors.ClassError) *jsonapi.Error {
	// check if the class is stored in the mapper
	creator, ok := c.Class[e.Class()]
	if !ok {
		// otherwise check it's minor
		creator, ok = c.Minors[e.Class().Minor()]
		if !ok {
			// at last check it's major
			creator, ok = c.Majors[e.Class().Major()]
			if !ok {
				log.Errorf("Unmapped error proivded: %v, with Class: %v", e, e.Class())
				return ErrInternalError()
			}
		}
	}

	err := creator()
	err.Code = strconv.FormatInt(int64(e.Class()), 16)
	detailed, ok := e.(errors.DetailedError)
	if ok {
		err.Detail = detailed.Details()
		err.ID = detailed.ID().String()
	}
	return err
}
