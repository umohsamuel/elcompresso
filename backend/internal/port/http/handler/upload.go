package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/response"
)

type UploadHandlerDependencies struct {
	Env     env.EnvironmentVariables
	Adapter adapter.Adapters
}

type UploadHandler struct {
	env     env.EnvironmentVariables
	adapter adapter.Adapters
}

func NewUploadHandler(deps UploadHandlerDependencies) UploadHandler {
	return UploadHandler{
		env:     deps.Env,
		adapter: deps.Adapter,
	}
}

func (s *UploadHandler) UploadHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("File is required"), http.StatusBadRequest).Send(c)
		return
	}

	if file.Size > 1000<<20 {
		response.NewErrorResponse(fmt.Errorf("file too large"), http.StatusBadRequest).Send(c)
		return
	}

	src, err := file.Open()
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("Failed to open file")).Send(c)

		return
	}
	defer src.Close()

	key := uuid.New().String() + "_" + file.Filename

	url, err := s.adapter.Storage.Upload(c.Request.Context(), key, src)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("upload failed: %w", err)).Send(c)
		return
	}

	response.NewSuccessResponse("success", gin.H{"url": url}, nil).Send(c)
}
