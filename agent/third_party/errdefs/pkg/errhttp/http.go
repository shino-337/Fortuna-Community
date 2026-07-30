package errhttp

import (
	"net/http"

	"github.com/containerd/errdefs"
)

// ToNative maps HTTP status codes to containerd errdefs types.
// Returns nil if no mapping is appropriate.
func ToNative(statusCode int) error {
	switch statusCode {
	case http.StatusBadRequest:
		return errdefs.ErrInvalidArgument
	case http.StatusUnauthorized:
		return errdefs.ErrUnauthenticated
	case http.StatusForbidden:
		return errdefs.ErrPermissionDenied
	case http.StatusNotFound:
		return errdefs.ErrNotFound
	case http.StatusConflict:
		return errdefs.ErrAlreadyExists
	case http.StatusRequestedRangeNotSatisfiable:
		return errdefs.ErrOutOfRange
	case http.StatusNotImplemented:
		return errdefs.ErrNotImplemented
	case http.StatusServiceUnavailable:
		return errdefs.ErrUnavailable
	default:
		if statusCode >= http.StatusInternalServerError {
			return errdefs.ErrInternal
		}
	}
	return nil
}
