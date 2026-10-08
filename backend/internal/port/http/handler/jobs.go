package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/umohsamuel/elcompresso/internal/adapter/storage"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	"github.com/umohsamuel/elcompresso/internal/domain/job"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/response"
)

type JobHandler struct {
	env      *env.EnvironmentVariables
	services *service.Services
}

func NewJobHandler(e *env.EnvironmentVariables, s *service.Services) *JobHandler {
	return &JobHandler{env: e, services: s}
}

func (h *JobHandler) limit(t compress.FileType) int64 {
	switch t {
	case compress.FileTypeVideo:
		return h.env.Limits.VideoBytes
	case compress.FileTypeAudio:
		return h.env.Limits.AudioBytes
	default:
		return h.env.Limits.ImageBytes
	}
}

type beginUploadRequest struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// BeginUpload hands out a presigned form for uploading one file straight to
// storage. The browser posts the file there, then calls Create with the
// upload id. Only available when Capabilities reports direct_upload.
//
// POST /api/v1/uploads/:type  {"name": "clip.mp4", "size": 123}
func (h *JobHandler) BeginUpload(c *gin.Context) {
	uploads := h.services.Uploads
	if uploads == nil {
		response.NewErrorResponse(errors.New("direct uploads are not enabled on this server"), http.StatusNotFound).Send(c)
		return
	}
	fileType := compress.FileType(c.Param("type"))
	if !fileType.Valid() {
		response.NewErrorResponse(fmt.Errorf("unknown file type %q", fileType), http.StatusNotFound).Send(c)
		return
	}
	clientIP := c.ClientIP()
	if err := h.services.Jobs.CheckCapacity(c.Request.Context(), clientIP); err != nil {
		sendJobError(c, err)
		return
	}

	var req beginUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.NewErrorResponse(errors.New("expected {\"name\", \"size\"}"), http.StatusBadRequest).Send(c)
		return
	}
	limit := h.limit(fileType)
	switch {
	case req.Size <= 0:
		response.NewErrorResponse(errors.New("a non-empty file is required"), http.StatusBadRequest).Send(c)
		return
	case req.Size > limit:
		response.NewErrorResponse(fmt.Errorf("file too large: the %s limit is %d MB", fileType, limit>>20), http.StatusRequestEntityTooLarge).Send(c)
		return
	}

	u, err := uploads.Begin(c.Request.Context(), fileType, clientIP, cleanName(req.Name), limit)
	if err != nil {
		response.NewErrorResponse(errors.New("could not start the upload")).Send(c)
		return
	}
	response.NewSuccessResponse("upload ready", u, nil).SendStatus(c, http.StatusCreated)
}

type createFromUploadRequest struct {
	UploadID string          `json:"upload_id"`
	Options  json.RawMessage `json:"options"`
}

// Create queues a compression job. It accepts either a multipart upload with
// an "options" JSON field and a "file" field, or JSON {"upload_id", "options"}
// for a file already uploaded through BeginUpload.
//
// POST /api/v1/jobs/:type
func (h *JobHandler) Create(c *gin.Context) {
	fileType := compress.FileType(c.Param("type"))
	if !fileType.Valid() {
		response.NewErrorResponse(fmt.Errorf("unknown file type %q", fileType), http.StatusNotFound).Send(c)
		return
	}

	jobs := h.services.Jobs
	clientIP := c.ClientIP()
	if err := jobs.CheckCapacity(c.Request.Context(), clientIP); err != nil {
		sendJobError(c, err)
		return
	}

	if c.ContentType() == "application/json" {
		h.createFromUpload(c, fileType, clientIP)
		return
	}

	limit := h.limit(fileType)
	tooLarge := fmt.Errorf("file too large: the %s limit is %d MB", fileType, limit>>20)
	const formOverhead = 1 << 20
	if c.Request.ContentLength > limit+formOverhead {
		response.NewErrorResponse(tooLarge, http.StatusRequestEntityTooLarge).Send(c)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit+formOverhead)

	reader, err := c.Request.MultipartReader()
	if err != nil {
		response.NewErrorResponse(errors.New("expected a multipart/form-data upload"), http.StatusBadRequest).Send(c)
		return
	}

	id := uuid.NewString()
	workDir, err := jobs.PrepareWorkDir(id)
	if err != nil {
		response.NewErrorResponse(errors.New("could not prepare job")).Send(c)
		return
	}
	submitted := false
	defer func() {
		if !submitted {
			_ = os.RemoveAll(workDir)
		}
	}()

	var (
		rawOptions []byte
		fileName   string
		inputPath  string
		size       int64
	)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			sendUploadError(c, err, tooLarge)
			return
		}

		switch part.FormName() {
		case "options":
			rawOptions, err = io.ReadAll(io.LimitReader(part, 64<<10))
		case "file":
			if inputPath != "" {
				err = errors.New("send one file per job")
				break
			}
			fileName = part.FileName()
			inputPath = filepath.Join(workDir, "input"+safeExt(fileName))
			size, err = saveUpload(part, inputPath, limit)
			if err == nil && size > limit {
				err = tooLarge
			}
		}
		part.Close()
		if err != nil {
			sendUploadError(c, err, tooLarge)
			return
		}
	}

	if inputPath == "" || size == 0 {
		response.NewErrorResponse(errors.New("a non-empty file is required"), http.StatusBadRequest).Send(c)
		return
	}

	opts, err := compress.ParseOptions(fileType, rawOptions, h.services.Capabilities)
	if err != nil {
		response.NewErrorResponse(err, http.StatusBadRequest).Send(c)
		return
	}

	j, err := jobs.Submit(c.Request.Context(), service.SubmitRequest{
		ID:           id,
		Type:         fileType,
		ClientIP:     clientIP,
		InputPath:    inputPath,
		WorkDir:      workDir,
		OriginalName: cleanName(fileName),
		OriginalSize: size,
		Options:      opts,
	})
	if err != nil {
		sendJobError(c, err)
		return
	}
	submitted = true
	response.NewSuccessResponse("job queued", j, nil).SendStatus(c, http.StatusAccepted)
}

func (h *JobHandler) createFromUpload(c *gin.Context, fileType compress.FileType, clientIP string) {
	uploads := h.services.Uploads
	if uploads == nil {
		response.NewErrorResponse(errors.New("direct uploads are not enabled on this server"), http.StatusBadRequest).Send(c)
		return
	}
	var req createFromUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UploadID == "" {
		response.NewErrorResponse(errors.New("expected {\"upload_id\", \"options\"}"), http.StatusBadRequest).Send(c)
		return
	}
	// Validate options before claiming the upload, so a bad request can be
	// fixed and retried with the same upload.
	opts, err := compress.ParseOptions(fileType, req.Options, h.services.Capabilities)
	if err != nil {
		response.NewErrorResponse(err, http.StatusBadRequest).Send(c)
		return
	}

	up, err := uploads.Take(c.Request.Context(), req.UploadID, fileType, clientIP)
	if err != nil {
		if errors.Is(err, service.ErrUploadNotFound) || errors.Is(err, storage.ErrNotFound) {
			response.NewErrorResponse(service.ErrUploadNotFound, http.StatusNotFound).Send(c)
			return
		}
		response.NewErrorResponse(errors.New("could not read the upload")).Send(c)
		return
	}

	id := uuid.NewString()
	workDir, err := h.services.Jobs.PrepareWorkDir(id)
	if err != nil {
		response.NewErrorResponse(errors.New("could not prepare job")).Send(c)
		return
	}
	j, err := h.services.Jobs.Submit(c.Request.Context(), service.SubmitRequest{
		ID:           id,
		Type:         fileType,
		ClientIP:     clientIP,
		InputPath:    filepath.Join(workDir, "input"+safeExt(up.Name)),
		WorkDir:      workDir,
		OriginalName: up.Name,
		OriginalSize: up.Size,
		Options:      opts,
		UploadKey:    up.Key,
	})
	if err != nil {
		_ = os.RemoveAll(workDir)
		_ = uploads.Discard(c.Request.Context(), up.Key)
		sendJobError(c, err)
		return
	}
	response.NewSuccessResponse("job queued", j, nil).SendStatus(c, http.StatusAccepted)
}

// cleanName keeps only the base name of a client-supplied file name.
func cleanName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || name == "/" {
		return ""
	}
	return name
}

// Get returns a job's status.
//
// GET /api/v1/jobs/:id
func (h *JobHandler) Get(c *gin.Context) {
	j, err := h.services.Jobs.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		sendJobError(c, err)
		return
	}
	response.NewSuccessResponse("success", j, nil).Send(c)
}

// List returns several jobs at once so clients can poll a batch in one call.
// Unknown or expired ids are omitted.
//
// GET /api/v1/jobs?ids=a,b,c
func (h *JobHandler) List(c *gin.Context) {
	ids := strings.Split(c.Query("ids"), ",")
	if len(ids) > 100 {
		response.NewErrorResponse(errors.New("at most 100 ids per request"), http.StatusBadRequest).Send(c)
		return
	}
	out := make([]job.Job, 0, len(ids))
	for _, id := range ids {
		if j, err := h.services.Jobs.Get(c.Request.Context(), strings.TrimSpace(id)); err == nil {
			out = append(out, j)
		}
	}
	response.NewSuccessResponse("success", out, nil).Send(c)
}

// Cancel stops a pending job, or deletes a finished job's output.
//
// DELETE /api/v1/jobs/:id
func (h *JobHandler) Cancel(c *gin.Context) {
	if err := h.services.Jobs.Cancel(c.Request.Context(), c.Param("id")); err != nil {
		sendJobError(c, err)
		return
	}
	response.NewSuccessResponse("canceled", nil, nil).Send(c)
}

// Download streams the result (local storage) or redirects to a presigned URL.
// Add ?inline=1 to display it in the browser instead of downloading.
//
// GET /api/v1/jobs/:id/download
func (h *JobHandler) Download(c *gin.Context) {
	inline := c.Query("inline") == "1"
	d, err := h.services.Jobs.Download(c.Request.Context(), c.Param("id"), inline)
	if err != nil {
		sendJobError(c, err)
		return
	}
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	defer d.Object.Body.Close()
	c.Header("Content-Disposition", storage.ContentDisposition(d.FileName, inline))
	c.Header("Content-Type", d.Type)
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, d.FileName, d.Object.ModTime, d.Object.Body)
}

func saveUpload(src io.Reader, path string, limit int64) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, errors.New("could not store upload")
	}
	n, err := io.Copy(f, io.LimitReader(src, limit+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return n, err
}

var extRe = regexp.MustCompile(`^\.[a-z0-9]{1,8}$`)

// safeExt keeps the upload's extension (ffmpeg uses it as a format hint) only
// if it is a plain alphanumeric extension.
func safeExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if extRe.MatchString(ext) {
		return ext
	}
	return ".bin"
}

func sendUploadError(c *gin.Context, err, tooLarge error) {
	var maxErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxErr) || errors.Is(err, tooLarge):
		response.NewErrorResponse(tooLarge, http.StatusRequestEntityTooLarge).Send(c)
	default:
		response.NewErrorResponse(fmt.Errorf("upload failed: %w", err), http.StatusBadRequest).Send(c)
	}
}

func sendJobError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrTooManyJobs):
		status = http.StatusTooManyRequests
	case errors.Is(err, service.ErrQueueFull), errors.Is(err, service.ErrShuttingDown):
		c.Header("Retry-After", "60")
		status = http.StatusServiceUnavailable
	case errors.Is(err, service.ErrNotReady):
		status = http.StatusConflict
	}
	response.NewErrorResponse(err, status).Send(c)
}
