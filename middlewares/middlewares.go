package middlewares

import (
	"net/http"

	"github.com/neuronlabs/jsonapi"
)

// Middleware is the function used as a http.Handler.
type Middleware func(next http.Handler) http.Handler

// compile time check for the Middleware.
var _ Middleware = AcceptMediaTypeMid

// AcceptMediaTypeMid is the middleware that checks if the request contains
// Header "Accept: application/vnd.api+json".
func AcceptMediaTypeMid(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mediaType := req.Header.Get("Accept")
		if mediaType != jsonapi.MediaType {
			rw.WriteHeader(http.StatusNotAcceptable)
			return
		}
		next.ServeHTTP(rw, req)
	})
}

// compile time check for the Middleware.
var _ Middleware = UnsupportedMediaTypeMid

// UnsupportedMediaTypeMid is the middleware that checks if the request contains Header "Content-Type" with
// media type different then `application/vnd.api+json`
func UnsupportedMediaTypeMid(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mediaType := req.Header.Get("Content-Type")
		if mediaType != jsonapi.MediaType {
			rw.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(rw, req)
	})
}
