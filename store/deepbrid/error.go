package deepbrid

import (
	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/store"
)

func UpstreamErrorWithCause(cause error) *core.UpstreamError {
	err := core.NewUpstreamError("")
	err.StoreName = string(store.StoreNameDeepBrid)

	if rerr, ok := cause.(*ResponseError); ok {
		if rerr.Message != "" {
			err.Msg = rerr.Message
		} else {
			err.Msg = "DeepBrid Error"
		}
		err.Code = translateHTTPErrorCode(rerr.ErrorCode)
		err.UpstreamCause = rerr
	} else {
		err.Cause = cause
	}

	return err
}

func translateHTTPErrorCode(statusCode int) core.ErrorCode {
	switch statusCode {
	case 401:
		return core.ErrorCodeUnauthorized
	case 403:
		return core.ErrorCodeForbidden
	case 429:
		return core.ErrorCodeTooManyRequests
	case 500:
		return core.ErrorCodeInternalServerError
	default:
		return core.ErrorCodeUnknown
	}
}
