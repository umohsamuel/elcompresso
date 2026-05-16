package compress

type Interface interface {
	Compress(req CompressionRequest) (*CompressionResult, error)
	Supports(fileType FileType, extension string) bool
}
