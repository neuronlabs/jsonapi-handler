package class

import (
	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/neuron-core/class"
)

var (
	// MjrHandler is the major router error classification.
	MjrHandler errors.Major
	// MjrGateway is the major gateway error classification.
	MjrGateway errors.Major
	// MjrMiddleware is the major error classification for middlewares.
	MjrMiddleware errors.Major
	// MjrConfig is the major error classification for config related issues.
	MjrConfig errors.Major
)

func init() {
	registerGatewayClasses()
	registerHandlerClasses()
	registerMiddlewareClasses()
	registerConfigClasses()
	registerQueryClasses()

}

var (
	// HandlerNotRegistered is the error classification for the
	// not registered routers.
	HandlerNotRegistered errors.Class

	// HandlerAlreadyRegistered is the error classification when the
	// router with given name is already registered.
	HandlerAlreadyRegistered errors.Class

	// HandlerConfigNoName is the error classification for the router
	// config without router name.
	HandlerConfigNoName errors.Class
)

func registerHandlerClasses() {
	MjrHandler = errors.MustNewMajor()

	MnrHandlerRegister := errors.MustNewMinor(MjrHandler)
	IndexHandlerNotRegistered := errors.MustNewIndex(MjrHandler, MnrHandlerRegister)
	HandlerNotRegistered = errors.MustNewClass(MjrHandler, MnrHandlerRegister, IndexHandlerNotRegistered)

	IndexHandlerAlreadyRegistered := errors.MustNewIndex(MjrHandler, MnrHandlerRegister)
	HandlerAlreadyRegistered = errors.MustNewClass(MjrHandler, MnrHandlerRegister, IndexHandlerAlreadyRegistered)

	MnrHandlerConfig := errors.MustNewMinor(MjrHandler)
	IndexHandlerConfigNoName := errors.MustNewIndex(MjrHandler, MnrHandlerConfig)
	HandlerConfigNoName = errors.MustNewClass(MjrHandler, MnrHandlerRegister, IndexHandlerConfigNoName)
}

func registerGatewayClasses() {
	MjrGateway = errors.MustNewMajor()
}

var (
	// MiddlewareAlreadyRegistered is the error classification for middlewares that are already registered
	// for provided name.
	MiddlewareAlreadyRegistered errors.Class
	// MiddlewareNotRegistered is the error classification for the middleware
	// that is not registered.
	MiddlewareNotRegistered errors.Class
)

func registerMiddlewareClasses() {
	MjrMiddleware = errors.MustNewMajor()

	mnrAlreadyRegistered := errors.MustNewMinor(MjrMiddleware)
	MiddlewareAlreadyRegistered = errors.MustNewMinorClass(MjrMiddleware, mnrAlreadyRegistered)

	mnrNotRegistered := errors.MustNewMinor(MjrMiddleware)
	MiddlewareNotRegistered = errors.MustNewMinorClass(MjrMiddleware, mnrNotRegistered)
}

var (
	// MnrConfigModel minor error classification for config models issues.
	MnrConfigModel errors.Minor

	// ConfigModelRelatedEndpoint is the error classification for config model endpoint issues.
	ConfigModelRelatedEndpoint errors.Class
)

func registerConfigClasses() {
	MjrConfig = errors.MustNewMajor()
	MnrConfigModel = errors.MustNewMinor(MjrConfig)

	configModelRE := errors.MustNewIndex(MjrConfig, MnrConfigModel)
	ConfigModelRelatedEndpoint = errors.MustNewClass(MjrConfig, MnrConfigModel, configModelRE)
}

var (
	// MnrQueryParameter is the minor error classification for invalid client queries parameters.
	MnrQueryParameter errors.Minor

	// QueryInvalidParameter is the error classification for invalid url queries parameters.
	QueryInvalidParameter errors.Class

	// MnrQueryTimeout is the minor error classification for timed out client queries.
	MnrQueryTimeout errors.Minor

	// QueryTimeout is the error class for timed out client queries.
	QueryTimeout errors.Class

	// MnrQueryURL is the minor error classification for query urls.
	MnrQueryURL errors.Minor

	// QueryInvalidURL is the error classification for invalid query urls.
	QueryInvalidURL errors.Class
)

func registerQueryClasses() {
	MnrQueryParameter = errors.MustNewMinor(class.MjrQuery)

	invalidParameter := errors.MustNewIndex(class.MjrQuery, MnrQueryParameter)
	QueryInvalidParameter = errors.MustNewClass(class.MjrQuery, MnrQueryParameter, invalidParameter)

	MnrQueryTimeout = errors.MustNewMinor(class.MjrQuery)
	QueryTimeout = errors.MustNewMinorClass(class.MjrQuery, MnrQueryTimeout)

	MnrQueryURL = errors.MustNewMinor(class.MjrQuery)
	QueryInvalidURL = errors.MustNewMinorClass(class.MjrQuery, MnrQueryURL)
}
