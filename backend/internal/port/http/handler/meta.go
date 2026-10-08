package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/response"
)

type MetaHandler struct {
	env      *env.EnvironmentVariables
	services *service.Services
}

func NewMetaHandler(e *env.EnvironmentVariables, s *service.Services) *MetaHandler {
	return &MetaHandler{env: e, services: s}
}

type limitsResponse struct {
	VideoBytes       int64 `json:"video_bytes"`
	AudioBytes       int64 `json:"audio_bytes"`
	ImageBytes       int64 `json:"image_bytes"`
	ImageMegapixels  int64 `json:"image_megapixels"`
	MaxActiveJobs    int   `json:"max_active_jobs"`
	ResultTTLSeconds int   `json:"result_ttl_seconds"`
}

// Capabilities tells the frontend what this server can do so it only offers
// options that will work.
//
// GET /api/v1/capabilities
func (h *MetaHandler) Capabilities(c *gin.Context) {
	l := h.env.Limits
	response.NewSuccessResponse("success", gin.H{
		"formats": h.services.Capabilities,
		"limits": limitsResponse{
			VideoBytes:       l.VideoBytes,
			AudioBytes:       l.AudioBytes,
			ImageBytes:       l.ImageBytes,
			ImageMegapixels:  l.ImagePixels / 1_000_000,
			MaxActiveJobs:    h.env.MaxActiveJobsPerIP,
			ResultTTLSeconds: int(h.env.ResultTTL.Seconds()),
		},
		"container_codecs": compress.ContainerCodecs,
		// Browsers upload straight to storage (S3) instead of through the API.
		"direct_upload": h.services.Uploads != nil,
		"defaults": gin.H{
			"video": compress.DefaultVideoOptions(),
			"audio": compress.DefaultAudioOptions(),
			"image": compress.DefaultImageOptions(),
		},
	}, nil).Send(c)
}

// Health reports liveness plus queue load.
//
// GET /health
func (h *MetaHandler) Health(c *gin.Context) {
	response.NewSuccessResponse("server up!!!", gin.H{"queue": h.services.Jobs.Stats()}, nil).Send(c)
}
