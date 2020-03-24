package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseAcceptEncoding tests ParseAcceptEncoding function.
func TestParseAcceptEncoding(t *testing.T) {
	t.Run("Mixed", func(t *testing.T) {
		h := http.Header{"Accept-Encoding": {"deflate, gzip;q=1.0, *;q=0.5"}}

		qv := ParseAcceptEncoding(h)

		if assert.Len(t, qv, 3) {
			assert.Equal(t, "deflate", qv[0].Value)
			assert.Equal(t, 1.0, qv[0].Quality)

			assert.Equal(t, "gzip", qv[1].Value)
			assert.Equal(t, 1.0, qv[1].Quality)

			assert.Equal(t, "*", qv[2].Value)
			assert.Equal(t, 0.5, qv[2].Quality)
		}
	})
}
