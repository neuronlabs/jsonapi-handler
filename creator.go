package handler

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"path"
	"reflect"

	"github.com/neuronlabs/brotli"
	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/jsonapi"
	"github.com/neuronlabs/neuron-core/class"
	"github.com/neuronlabs/neuron-core/controller"
	"github.com/neuronlabs/neuron-core/mapping"
	"github.com/neuronlabs/neuron-core/query"

	handlerErrors "github.com/neuronlabs/jsonapi-handler/errors"
	"github.com/neuronlabs/jsonapi-handler/log"
)

// Creator is the neuron gateway handler that implements
// https://jsonapi.org server routes for neuron models.
type Creator struct {
	BasePath string
	// DefaultPageSize defines default PageSize for the list endpoints.
	DefaultPageSize int
	// NoContentOnCreate allows to set the flag for the models with client generated id to return no content.
	NoContentOnCreate bool
	// CompressionLevel defines the compression level for the handler function writers
	CompressionLevel int
	// StrictQueriesMode if true sets the strict mode for the query builder, that doesn't allow
	// unknown query keys, and unknown fields.
	StrictQueriesMode bool
	// StrictFieldsMode defines if the during unmarshal process the query should strictly check
	// if all the fields are well known to given model.
	StrictFieldsMode bool
	// QueryErrorsLimit defines the upper limit of the error number while getting the query.
	QueryErrorsLimit int
	// IncludeNestedLimit is a maximum value for nested includes (i.e. IncludeNestedLimit = 1
	// allows ?include=posts.comments but does not allow ?include=posts.comments.author)
	IncludeNestedLimit int
	// FilterValueLimit is a maximum length of the filter values
	FilterValueLimit int
	// MarshalLinks is the default behavior for marshaling the resource links into the handler responses.
	MarshalLinks bool
	c            *controller.Controller
}

// New creates new jsonapi Creator.
func NewC(c *controller.Controller) *Creator {
	return &Creator{
		QueryErrorsLimit: 10,
		c:                c,
	}
}

func (h *Creator) basePath() string {
	if h.BasePath == "" {
		return "/"
	}
	return h.BasePath
}

func (h *Creator) baseModelPath(mStruct *mapping.ModelStruct) string {
	return path.Join("/", h.BasePath, mStruct.Collection())
}

func (h *Creator) writeContentType(rw http.ResponseWriter) {
	rw.Header().Add("Content-Type", jsonapi.MediaType)
}

func (h *Creator) jsonapiUnmarshalOptions() *jsonapi.UnmarshalOptions {
	return &jsonapi.UnmarshalOptions{StrictUnmarshalMode: h.StrictFieldsMode}
}

func (h *Creator) getFieldValue(modelValue interface{}, field *mapping.StructField) (value interface{}, err error) {
	if modelValue == nil {
		return nil, errors.New(class.ModelValueNil, "provided nil value")
	}
	v := reflect.ValueOf(modelValue).Elem()
	return v.FieldByIndex(field.ReflectField().Index).Interface(), nil
}

func (h *Creator) marshalErrors(rw http.ResponseWriter, req *http.Request, status int, errs ...*jsonapi.Error) {
	h.writeContentType(rw)

	if status == 0 {
		status = handlerErrors.MultiError(errs).Status()
	}
	rw.WriteHeader(status)

	w := h.writer(rw, req)
	defer func() {
		wc, ok := w.(io.WriteCloser)
		if ok {
			if err := wc.Close(); err != nil {
				log.Debugf("Closing Writer failed: %v", err)
			}

		}
	}()

	err := jsonapi.MarshalErrors(w, errs...)
	if err != nil {
		log.Errorf("Marshaling errors: '%v' failed: %v", errs, err)
	}
}

func (h *Creator) marshalScope(s *query.Scope, rw http.ResponseWriter, req *http.Request, status int, option ...*jsonapi.MarshalOptions) {
	h.writeContentType(rw)
	w := h.writer(rw, req)
	defer func() {
		wc, ok := w.(io.WriteCloser)
		if ok {
			if err := wc.Close(); err != nil {
				log.Debugf("Close failed: %v", err)
			}
		}
	}()

	rw.WriteHeader(status)

	if err := jsonapi.MarshalScope(w, s, option...); err != nil {
		log.Errorf("[SCOPE][%s] jsonapi.MarshalScope failed: %v", s.ID().String(), err)
		err := jsonapi.MarshalErrors(w, handlerErrors.ErrInternalError())
		if err != nil {
			switch err {
			case io.ErrShortWrite, io.ErrClosedPipe:
				log.Debug2f("An error occurred while writing api errors: %v", err)
				return
			default:
				log.Errorf("Marshaling error failed: %v", err)
			}
		}
	}
}

func (h *Creator) writer(rw http.ResponseWriter, req *http.Request) io.Writer {
	accepts := ParseAcceptEncoding(req.Header)

	w := io.Writer(rw)
	var err error

	for _, accept := range accepts {
		compressionLevel := h.CompressionLevel
		switch accept.Value {
		case "gzip":
			switch {
			case compressionLevel > gzip.BestCompression:
				compressionLevel = gzip.BestCompression
			case compressionLevel < gzip.BestSpeed:
				compressionLevel = gzip.BestSpeed
			case compressionLevel == -1:
				compressionLevel = gzip.DefaultCompression
			}
			w, err = gzip.NewWriterLevel(rw, h.CompressionLevel)
		case "deflate":
			switch {
			case compressionLevel > flate.BestCompression:
				compressionLevel = flate.BestCompression
			case compressionLevel < flate.BestSpeed:
				compressionLevel = flate.BestSpeed
			case compressionLevel == -1:
				compressionLevel = flate.DefaultCompression
			}
			w, err = flate.NewWriter(rw, h.CompressionLevel)
		case "br":
			switch {
			case h.CompressionLevel > brotli.BestCompression:
				compressionLevel = brotli.BestCompression
			case h.CompressionLevel < brotli.BestSpeed:
				compressionLevel = brotli.BestSpeed
			case compressionLevel == -1:
				compressionLevel = brotli.DefaultCompression
			}
			w = brotli.NewWriterLevel(rw, compressionLevel)
		default:
			continue
		}
		if log.Level() == log.LDEBUG3 {
			log.Debug3f("Writer: '%s' with compression level: %d", accept.Value, compressionLevel)
		}
		rw.Header().Set("Content-Encoding", accept.Value)
		break
	}

	if err != nil {
		log.Warningf("Can't create compressed writer: %v", err)
		w = rw
	}
	return w
}
