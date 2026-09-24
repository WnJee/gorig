package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	configure "github.com/WnJee/gorig/utils/cofigure"
)

type DriverType string

const (
	MinIO DriverType = "minio"
	S3    DriverType = "s3"
	OSS   DriverType = "oss"
	Local DriverType = "local"
)

// ObjectInfo represents object metadata in storage.
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
	ContentType  string    `json:"content_type"`
}

// PutResult represents the result of an upload.
type PutResult struct {
	Key   string `json:"key"`
	ETag  string `json:"etag"`
	Size  int64  `json:"size"`
	URL   string `json:"url"`
}

// MultipartUpload represents an active multipart upload session.
type MultipartUpload struct {
	UploadID string `json:"upload_id"`
	Key      string `json:"key"`
	Bucket   string `json:"bucket"`
}

// Part represents an uploaded part in a multipart upload.
type Part struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// ResumableOptions holds configuration for resumable uploads.
type ResumableOptions struct {
	PartSize       int64                                // Part size in bytes (default: 5MB)
	Concurrency    int                                  // Number of concurrent upload workers (default: 3)
	CheckpointPath string                               // File path for saving resumable checkpoint (default: <localFile>.cp)
	Progress       func(uploadedBytes, totalBytes int64) // Progress callback
	ContentType    string                               // MIME Content-Type
}

// Storage is the unified storage abstraction interface.
type Storage interface {
	// Put uploads an object with an io.Reader stream.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (*PutResult, error)
	// PutBytes uploads byte slice directly.
	PutBytes(ctx context.Context, key string, data []byte, contentType string) (*PutResult, error)
	// PutFile uploads a file from local filesystem.
	PutFile(ctx context.Context, key string, localFilePath string) (*PutResult, error)
	// Get retrieves an object reader stream. Caller MUST close the reader.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// GetBytes retrieves the full object content as byte slice.
	GetBytes(ctx context.Context, key string) ([]byte, error)
	// GetURL generates a public or presigned access URL for the key.
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
	// Delete removes an object by key.
	Delete(ctx context.Context, key string) error
	// DeleteMulti removes multiple objects by keys.
	DeleteMulti(ctx context.Context, keys []string) error
	// Exists checks if an object exists.
	Exists(ctx context.Context, key string) (bool, error)
	// Stat returns object metadata.
	Stat(ctx context.Context, key string) (*ObjectInfo, error)

	// Multipart upload methods
	InitiateMultipartUpload(ctx context.Context, key string, contentType string) (*MultipartUpload, error)
	UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int, r io.Reader, partSize int64) (*Part, error)
	CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload, parts []*Part) (*PutResult, error)
	AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error
	ListParts(ctx context.Context, upload *MultipartUpload) ([]*Part, error)

	// Resumable upload method
	UploadFileResumable(ctx context.Context, key string, localFilePath string, opts ...*ResumableOptions) (*PutResult, error)
}

// Config represents the common configuration for storage drivers.
type Config struct {
	Driver          DriverType `json:"driver"`
	Endpoint        string     `json:"endpoint"`
	AccessKeyID     string     `json:"access_key_id"`
	AccessKeySecret string     `json:"access_key_secret"`
	Bucket          string     `json:"bucket"`
	Region          string     `json:"region"`
	UseSSL          bool       `json:"use_ssl"`
	RootPath        string     `json:"root_path"`         // For local storage
	Domain          string     `json:"domain"`            // Custom CDN / public domain
	PathStyle       bool       `json:"path_style"`        // For MinIO / S3 path style
}

var (
	drivers   = make(map[DriverType]func(cfg *Config) (Storage, error))
	driversMu sync.RWMutex

	instances   = make(map[string]Storage)
	instancesMu sync.RWMutex
)

// RegisterDriver registers a storage factory for a driver type.
func RegisterDriver(driverType DriverType, factory func(cfg *Config) (Storage, error)) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[driverType] = factory
}

// New creates a new Storage instance from config.
func New(cfg *Config) (Storage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config is nil")
	}
	driver := DriverType(strings.ToLower(string(cfg.Driver)))
	driversMu.RLock()
	factory, ok := drivers[driver]
	driversMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.Driver)
	}
	return factory(cfg)
}

// LoadConfigFromEnv reads storage configuration from config files.
func LoadConfigFromEnv(name ...string) *Config {
	prefix := "storage"
	if len(name) > 0 && name[0] != "" {
		prefix = "storage." + name[0]
	}

	driverStr := configure.GetString(prefix+".driver", configure.GetString("storage.default", "local"))
	driver := DriverType(strings.ToLower(driverStr))

	return &Config{
		Driver:          driver,
		Endpoint:        configure.GetString(prefix + ".endpoint"),
		AccessKeyID:     configure.GetString(prefix + ".access_key_id"),
		AccessKeySecret: configure.GetString(prefix + ".access_key_secret"),
		Bucket:          configure.GetString(prefix + ".bucket"),
		Region:          configure.GetString(prefix + ".region", "us-east-1"),
		UseSSL:          configure.GetBool(prefix+".use_ssl", true),
		RootPath:        configure.GetString(prefix+".root_path", "./.storage/uploads/"),
		Domain:          configure.GetString(prefix + ".domain"),
		PathStyle:       configure.GetBool(prefix+".path_style", true),
	}
}

// GetStorage returns a singleton Storage instance by config name or default.
func GetStorage(name ...string) Storage {
	key := "default"
	if len(name) > 0 && name[0] != "" {
		key = name[0]
	}

	instancesMu.RLock()
	inst, ok := instances[key]
	instancesMu.RUnlock()
	if ok {
		return inst
	}

	instancesMu.Lock()
	defer instancesMu.Unlock()
	if inst, ok := instances[key]; ok {
		return inst
	}

	cfg := LoadConfigFromEnv(name...)
	storageInst, err := New(cfg)
	if err != nil {
		// Fallback to local storage if driver init failed
		localCfg := &Config{
			Driver:   Local,
			RootPath: "./.storage/uploads/",
		}
		storageInst, _ = New(localCfg)
	}
	instances[key] = storageInst
	return storageInst
}

// Global helper wrappers using default Storage

func Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (*PutResult, error) {
	return GetStorage().Put(ctx, key, r, size, contentType)
}

func PutBytes(ctx context.Context, key string, data []byte, contentType string) (*PutResult, error) {
	return GetStorage().PutBytes(ctx, key, data, contentType)
}

func PutFile(ctx context.Context, key string, localFilePath string) (*PutResult, error) {
	return GetStorage().PutFile(ctx, key, localFilePath)
}

func Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return GetStorage().Get(ctx, key)
}

func GetBytes(ctx context.Context, key string) ([]byte, error) {
	return GetStorage().GetBytes(ctx, key)
}

func GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return GetStorage().GetURL(ctx, key, expires)
}

func Delete(ctx context.Context, key string) error {
	return GetStorage().Delete(ctx, key)
}

func DeleteMulti(ctx context.Context, keys []string) error {
	return GetStorage().DeleteMulti(ctx, keys)
}

func Exists(ctx context.Context, key string) (bool, error) {
	return GetStorage().Exists(ctx, key)
}

func Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	return GetStorage().Stat(ctx, key)
}

func UploadFileResumable(ctx context.Context, key string, localFilePath string, opts ...*ResumableOptions) (*PutResult, error) {
	return GetStorage().UploadFileResumable(ctx, key, localFilePath, opts...)
}

// Helper methods on BaseStorage to reduce duplication across drivers

func PutBytesHelper(ctx context.Context, s Storage, key string, data []byte, contentType string) (*PutResult, error) {
	return s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType)
}

func PutFileHelper(ctx context.Context, s Storage, key string, localFilePath string) (*PutResult, error) {
	f, err := os.Open(localFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	contentType := detectContentType(localFilePath)
	return s.Put(ctx, key, f, stat.Size(), contentType)
}

func GetBytesHelper(ctx context.Context, s Storage, key string) ([]byte, error) {
	rc, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".json":
		return "application/json"
	case ".txt", ".log":
		return "text/plain; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
