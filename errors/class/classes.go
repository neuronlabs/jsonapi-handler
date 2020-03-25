package class

import (
	"github.com/neuronlabs/errors"
	"github.com/neuronlabs/neuron-core/class"
)

func init() {
	registerQueryClasses()
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
