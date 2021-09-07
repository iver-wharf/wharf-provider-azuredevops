package requests

import (
	"fmt"
	"net/http"
)

// Non2xxStatusError represents a failed request response where the HTTP status
// code was non-2xx, meaning not 200 (OK), not 201 (Created), etc.
type Non2xxStatusError struct {
	Status     string
	StatusCode int
}

// Error adds compliance to the error interface.
func (err Non2xxStatusError) Error() string {
	return fmt.Sprintf("non-2xx HTTP status: %s", err.Status)
}

func newNon2xxStatusError(resp *http.Response) error {
	return Non2xxStatusError{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}
}
