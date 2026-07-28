package putio

import (
	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/store"
)

func UpstreamErrorWithCause(cause error) *core.UpstreamError {
	err := core.NewUpstreamError("")
	err.StoreName = string(store.StoreNamePutIO)

	if rerr, ok := cause.(*ResponseError); ok {
		if rerr.ErrorMessage != "" {
			err.Msg = rerr.ErrorMessage
		} else {
			err.Msg = "Put.io Error: " + rerr.ErrorType
		}
		err.Code = translateErrorType(rerr.ErrorType)
		err.UpstreamCause = rerr
	} else {
		err.Cause = cause
	}

	return err
}

func translateErrorType(errorType string) core.ErrorCode {
	switch errorType {
	case "Unauthorized":
		return core.ErrorCodeUnauthorized
	case "NotFound":
		return core.ErrorCodeNotFound
	case "Forbidden":
		return core.ErrorCodeForbidden
	case "TooManyRequests":
		return core.ErrorCodeTooManyRequests
	default:
		return core.ErrorCodeUnknown
	}
}
