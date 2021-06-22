package ginutilext

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iver-wharf/wharf-core/pkg/ginutil"
	"github.com/iver-wharf/wharf-core/pkg/problem"
)

// WriteAPIReadError uses WriteProblemError to write a 502 "Bad Gateway"
// response with the type "/prob/api/unexpected-api-read-error".
//
// Meant to be used on unexpected error when reading data using the Wharf API.
func WriteAPIReadError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "/prob/provider/unexpected-api-read-error",
		Title: "Unexpected API read error.",
		Status: http.StatusBadGateway,
		Detail: detail,
	})
}

// WriteAPIWriteError uses WriteProblemError to write a 502 "Bad Gateway"
// response with the type "/prob/api/unexpected-api-write-error".
//
// Meant to be used on unexpected error when writing data using the Wharf API.
func WriteAPIWriteError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "/prob/provider/unexpected-api-write-error",
		Title: "Unexpected API write error.",
		Status: http.StatusBadGateway,
		Detail: detail,
	})
}

// WriteResponseFormatError uses WriteProblemError to write a 502 "Bad Gateway"
// response with the type "/prob/provider/unexpected-response-format".
//
// Meant to be used on unexpected error when the response format does not match our expectations.
func WriteResponseFormatError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "/prob/provider/unexpected-response-format",
		Title: "Unexpected response format.",
		Status: http.StatusBadGateway,
		Detail: detail,
	})
}

// WriteFetchBuildDefinitionError uses WriteProblemError to write a 400 "Bad Request"
// response with the type "/prob/provider/fetch-build-definition".
//
// Meant to be used on error when fetching the build definition.
func WriteFetchBuildDefinitionError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "/prob/provider/fetch-build-definition",
		Title: "Error fetching build definition.",
		Status: http.StatusBadRequest,
		Detail: detail,
	})
}

// WriteComposingProviderDataError uses WriteProblemError to write a 400 "Bad Request"
// response with the type "/prob/provider/composing-provider-data".
//
// Meant to be used on error when composing provider data.
func WriteComposingProviderDataError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "/prob/provider/composing-provider-data",
		Title: "Error composing provider data.",
		Status: http.StatusBadRequest,
		Detail: detail,
	})
}

// WriteTriggerError uses WriteProblemError to write a 502 "Bad Gateway"
// response with the type "/prob/build/unexpected-trigger-error".
//
// Meant to be used on unexpected error during build process regarding a trigger.
func WriteTriggerError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "/prob/build/unexpected-trigger-error",
		Title: "Unexpected trigger error.",
		Status: http.StatusBadGateway,
		Detail: detail,
	})
}

// WriteAuthenticationError uses WriteProblemError to write a 401 "Unauthorized"
// response with the type "prob/provider/authentication-error".
//
// Meant to be used for failed authentication.
func WriteAuthenticationError(c *gin.Context, err error, detail string) {
	ginutil.WriteProblemError(c, err, problem.Response{
		Type: "prob/provider/authentication-error",
		Title: "Error authenticating.",
		Status: http.StatusUnauthorized,
		Detail: detail,
	})
}