package errors

import (
	"strconv"

	"github.com/neuronlabs/jsonapi"

	"github.com/neuronlabs/jsonapi-handler/log"
)

// MultiError is the multiple Error wrapper.
type MultiError []*jsonapi.Error

// Status gets the most significant api error status.
func (m MultiError) Status() int {
	var highestStatus int
	for _, err := range m {
		status, er := strconv.Atoi(err.Status)
		if er != nil {
			log.Warningf("Error: '%v' contains non integer status value", err)
			continue
		}
		if err.Status == "500" {
			return 500
		}
		if status > highestStatus {
			highestStatus = status
		}
	}
	if highestStatus == 0 {
		highestStatus = 500
	}
	return highestStatus
}
