package ginsaas

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorHandler returns a generic error response without exposing tenant internals.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, errorBody("internal_error"))
	}
}
