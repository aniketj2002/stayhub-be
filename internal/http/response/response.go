package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Envelope struct {
	Data  any        `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data})
}

func Fail(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, Envelope{
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func BadRequest(c *gin.Context, code, message string, details any) {
	Fail(c, http.StatusBadRequest, code, message, details)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, "FORBIDDEN", message, nil)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, "NOT_FOUND", message, nil)
}

func Conflict(c *gin.Context, code, message string, details any) {
	Fail(c, http.StatusConflict, code, message, details)
}

func InternalError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", message, nil)
}
