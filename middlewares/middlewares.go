package middlewares

import (
	"net/http"
	"strings"

	"github.com/neuronlabs/jsonapi"
)

// Middleware is the function used as a http.Handler.
type Middleware func(next http.Handler) http.Handler

// compile time check for the Middleware.
var _ Middleware = AcceptMediaType

// AcceptMediaType is the middleware that checks if the request contains
// Header "Accept" with the value of: application/vnd.api+json".
func AcceptMediaType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mediaTypeHeader := req.Header.Get("Accept")
		mediaTypes := strings.Split(mediaTypeHeader, ",")
		for _, mediaType := range mediaTypes {
			if strings.TrimSpace(mediaType) == jsonapi.MediaType {
				next.ServeHTTP(rw, req)
				return
			}
		}
		rw.WriteHeader(http.StatusNotAcceptable)
	})
}

// compile time check for the Middleware.
var _ Middleware = UnsupportedMediaType

// UnsupportedMediaType is the middleware that checks if the request contains Header "Content-Type" with
// media type different then `application/vnd.api+json`
func UnsupportedMediaType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mediaType := req.Header.Get("Content-Type")
		if mediaType != jsonapi.MediaType {
			rw.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(rw, req)
	})
}
