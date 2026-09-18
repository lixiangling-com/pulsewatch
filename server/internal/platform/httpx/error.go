package httpx

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Code        string              `json:"code"`
	Message     string              `json:"message"`
	FieldErrors map[string][]string `json:"field_errors"`
	RequestID   string              `json:"request_id"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func WriteError(c *gin.Context, status int, code, message string, fieldErrors map[string][]string) {
	if fieldErrors == nil {
		fieldErrors = map[string][]string{}
	}
	c.AbortWithStatusJSON(status, ErrorResponse{Error: ErrorBody{
		Code:        code,
		Message:     message,
		FieldErrors: fieldErrors,
		RequestID:   c.GetString(RequestIDKey),
	}})
}
