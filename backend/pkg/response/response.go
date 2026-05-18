package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	StatusCode   int    `json:"statusCode"`
	Message      string `json:"message"`
	ErrorMessage any    `json:"error"`
}

func NewErrorResponse(err error, errCode ...int) ErrorResponse {
	var errorMessage string = ""
	if err != nil {
		errorMessage = err.Error()
	}

	statusCode := http.StatusInternalServerError
	if len(errCode) > 0 && errCode[0] != 0 {
		statusCode = errCode[0]
	}

	return ErrorResponse{
		StatusCode:   statusCode,
		Message:      http.StatusText(statusCode),
		ErrorMessage: errorMessage,
	}
}

func (res ErrorResponse) Send(c *gin.Context) {
	resJSON := map[string]any{
		"message": res.Message,
		"error":   res.ErrorMessage,
	}
	c.AbortWithStatusJSON(res.StatusCode, resJSON)
}

type SuccessResponse struct {
	StatusCode int         `json:"statusCode,omitempty"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data,omitempty"`
	Metadata   interface{} `json:"metadata,omitempty"`
}

func NewSuccessResponse(message string, data interface{}, metadata interface{}) *SuccessResponse {
	return &SuccessResponse{
		Message:  message,
		Data:     data,
		Metadata: metadata,
	}
}

func (res SuccessResponse) Send(c *gin.Context) {
	c.JSON(http.StatusOK, res)
}
