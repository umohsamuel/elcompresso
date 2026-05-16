package handler

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
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

func (h CompressHandler) CompressVideo(c *gin.Context) {

	file, err := c.FormFile("file")
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("invalid file: %v", err.Error())).Send(c)

		return
	}
	log.Println(file.Filename)

	if file.Size > 500<<20 {
		response.NewErrorResponse(fmt.Errorf("file too large: max 100MB")).Send(c)
		return
	}

	// dst := filepath.Join("./public/uploaded", filepath.Base(file.Filename))
	// c.SaveUploadedFile(file, dst)

	f, err := file.Open()
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to open file: %v", err)).Send(c)
		return
	}
	defer f.Close()

	req := compress.CompressionRequest{
		Input:    f,
		FileName: file.Filename,
		FileType: "video",
		Quality:  10,
	}

	res, err := h.adapter.Compressor.Video.Compress(req)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to compress file: %v", err)).Send(c)
		return
	}

	// cmpRes, err := json.Marshal(res)
	// if err != nil {
	// 	response.NewErrorResponse(fmt.Errorf("failed to parse json: %v", err)).Send(c)
	// 	return
	// }

	// Save compressed file to tmp/ directory
	outputName := "compressed_" + filepath.Base(file.Filename)
	outputPath := filepath.Join("tmp", outputName)

	outFile, err := os.Create(outputPath)
	if err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to save file: %v", err)).Send(c)
		return
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, res.Output); err != nil {
		response.NewErrorResponse(fmt.Errorf("failed to write file: %v", err)).Send(c)
		return
	}

	// Build download URL
	downloadLink := fmt.Sprintf("%s/downloads/%s", "", outputName)

	response.NewSuccessResponse("success", gin.H{
		"original_size":   file.Size,
		"compressed_size": res.CompressedSize,
		"download_link":   downloadLink,
	}, nil).Send(c)

}

func (h CompressHandler) CompressAudio(c *gin.Context) {

}

func (h CompressHandler) CompressImage(c *gin.Context) {

}
