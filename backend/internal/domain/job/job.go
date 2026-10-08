package job

import (
	"time"

	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
)

func (s Status) Finished() bool {
	return s == StatusDone || s == StatusFailed || s == StatusCanceled
}

type Result struct {
	Name         string             `json:"name"`
	Size         int64              `json:"size"`
	ContentType  string             `json:"content_type"`
	SavedPercent float64            `json:"saved_percent"`
	Larger       bool               `json:"larger"`
	DownloadURL  string             `json:"download_url"`
	Info         compress.MediaInfo `json:"info"`
	storageKey   string
}

func (r *Result) StorageKey() string       { return r.storageKey }
func (r *Result) SetStorageKey(key string) { r.storageKey = key }

// Job is the public, serialisable view of a compression job.
type Job struct {
	ID            string            `json:"id"`
	Type          compress.FileType `json:"type"`
	Status        Status            `json:"status"`
	Stage         string            `json:"stage,omitempty"`
	Progress      float64           `json:"progress"` // 0..100
	QueuePosition int               `json:"queue_position,omitempty"`
	Error         string            `json:"error,omitempty"`
	OriginalName  string            `json:"original_name"`
	OriginalSize  int64             `json:"original_size"`
	Options       compress.Options  `json:"options"`
	Result        *Result           `json:"result,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
}
