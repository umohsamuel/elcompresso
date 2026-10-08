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
}

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

// Record is a job plus the server-side details needed to run it. It is what
// the job store persists, so a job survives restarts of the API and workers.
type Record struct {
	Job       Job    `json:"job"`
	ClientIP  string `json:"client_ip"`
	InputPath string `json:"input_path"`
	WorkDir   string `json:"work_dir"`
	// UploadKey is set for direct uploads: the input is still in storage and
	// is fetched to InputPath when the job starts.
	UploadKey string `json:"upload_key,omitempty"`
	ResultKey string `json:"result_key,omitempty"`
	// Attempts counts how often a worker has started this job.
	Attempts int `json:"attempts"`
}
