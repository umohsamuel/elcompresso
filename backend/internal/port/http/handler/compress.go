package handler

import (
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/response"
)

type CompressHandlerDependencies struct {
	Env     env.EnvironmentVariables
	Adapter adapter.Adapters
}

type CompressHandler struct {
	env     env.EnvironmentVariables
	adapter adapter.Adapters
}

func NewCompressHandler(deps CompressHandlerDependencies) CompressHandler {
	return CompressHandler{
		env:     deps.Env,
		adapter: deps.Adapter,
	}
}

type FormData struct {
	File    *multipart.FileHeader `form:"file" binding:"required"`
	Quality int                   `form:"quality"`
}

func (h CompressHandler) CompressVideo(c *gin.Context) {

	var fData FormData

	if err := c.ShouldBind(&fData); err != nil {
		response.NewErrorResponse(fmt.Errorf("invalid form data: %v", err.Error())).Send(c)

		return
	}

	if fData.File.Size > 500<<20 {
		response.NewErrorResponse(fmt.Errorf("file too large: max 500MB")).Send(c)
		return
	}

	f, err := fData.File.Open()
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to open file: %v", err)).Send(c)
		return
	}
	defer f.Close()

	fmtedFileName := strings.ReplaceAll(fData.File.Filename, " ", "_")

	req := compress.CompressionRequest{
		Input:    f,
		FileName: fmtedFileName,
		FileType: "video",
		Quality:  fData.Quality,
	}

	res, err := h.adapter.Compressor.Video.Compress(req)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to compress file: %v", err)).Send(c)
		return
	}

	key, err := h.adapter.Storage.Upload(c.Request.Context(), uuid.New().String()+"_"+fData.File.Filename, res.Output)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("upload failed: %w", err)).Send(c)
		return
	}

	dUrl, err := h.adapter.Storage.GenerateDownloadURL(c.Request.Context(), key, 24*time.Hour)

	response.NewSuccessResponse("success", gin.H{
		"original_size":   fData.File.Size,
		"compressed_size": res.CompressedSize,
		"download_link":   dUrl,
	}, nil).Send(c)

}

func (h CompressHandler) CompressAudio(c *gin.Context) {
	var fData FormData

	if err := c.ShouldBind(&fData); err != nil {
		response.NewErrorResponse(fmt.Errorf("invalid form data: %v", err.Error())).Send(c)

		return
	}

	if fData.File.Size > 100<<20 {
		response.NewErrorResponse(fmt.Errorf("file too large: max 100MB")).Send(c)
		return
	}

	f, err := fData.File.Open()
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to open file: %v", err)).Send(c)
		return
	}
	defer f.Close()

	fmtedFileName := strings.ReplaceAll(fData.File.Filename, " ", "_")

	req := compress.CompressionRequest{
		Input:    f,
		FileName: fmtedFileName,
		FileType: "audio",
		Quality:  fData.Quality,
	}

	res, err := h.adapter.Compressor.Audio.Compress(req)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to compress file: %v", err)).Send(c)
		return
	}

	key, err := h.adapter.Storage.Upload(c.Request.Context(), uuid.New().String()+"_"+fData.File.Filename, res.Output)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("upload failed: %w", err)).Send(c)
		return
	}

	dUrl, err := h.adapter.Storage.GenerateDownloadURL(c.Request.Context(), key, 24*time.Hour)

	response.NewSuccessResponse("success", gin.H{
		"original_size":   fData.File.Size,
		"compressed_size": res.CompressedSize,
		"download_link":   dUrl,
	}, nil).Send(c)

}

func (h CompressHandler) CompressImage(c *gin.Context) {
	var fData FormData

	if err := c.ShouldBind(&fData); err != nil {
		response.NewErrorResponse(fmt.Errorf("invalid form data: %v", err.Error())).Send(c)

		return
	}

	if fData.File.Size > 100<<20 {
		response.NewErrorResponse(fmt.Errorf("file too large: max 100MB")).Send(c)
		return
	}

	f, err := fData.File.Open()
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to open file: %v", err)).Send(c)
		return
	}
	defer f.Close()

	fmtedFileName := strings.ReplaceAll(fData.File.Filename, " ", "_")

	req := compress.CompressionRequest{
		Input:    f,
		FileName: fmtedFileName,
		FileType: "image",
		Quality:  fData.Quality,
	}

	res, err := h.adapter.Compressor.Image.Compress(req)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to compress file: %v", err)).Send(c)
		return
	}

	key, err := h.adapter.Storage.Upload(c.Request.Context(), uuid.New().String()+"_"+fData.File.Filename, res.Output)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("upload failed: %w", err)).Send(c)
		return
	}

	dUrl, err := h.adapter.Storage.GenerateDownloadURL(c.Request.Context(), key, 24*time.Hour)

	response.NewSuccessResponse("success", gin.H{
		"original_size":   fData.File.Size,
		"compressed_size": res.CompressedSize,
		"download_link":   dUrl,
	}, nil).Send(c)
}
