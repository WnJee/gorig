package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"
)

const (
	DefaultPartSize    int64 = 5 * 1024 * 1024 // 5 MB
	DefaultConcurrency       = 3
)

// Checkpoint stores the state of an ongoing resumable upload.
type Checkpoint struct {
	UploadID   string          `json:"upload_id"`
	Key        string          `json:"key"`
	FilePath   string          `json:"file_path"`
	FileSize   int64           `json:"file_size"`
	FileMTime  int64           `json:"file_mtime"`
	PartSize   int64           `json:"part_size"`
	TotalParts int             `json:"total_parts"`
	Parts      map[int]*Part   `json:"parts"`
	Mu         sync.Mutex      `json:"-"`
}

func loadCheckpoint(cpPath string) (*Checkpoint, error) {
	data, err := os.ReadFile(cpPath)
	if err != nil {
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	if cp.Parts == nil {
		cp.Parts = make(map[int]*Part)
	}
	return &cp, nil
}

func (cp *Checkpoint) save(cpPath string) error {
	cp.Mu.Lock()
	defer cp.Mu.Unlock()
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return os.WriteFile(cpPath, data, 0644)
}

// UploadFileResumableGeneric provides standard resumable multipart file uploading across all storage backends.
func UploadFileResumableGeneric(ctx context.Context, s Storage, key string, localFilePath string, opts ...*ResumableOptions) (*PutResult, error) {
	file, err := os.Open(localFilePath)
	if err != nil {
		return nil, fmt.Errorf("open local file failed: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat local file failed: %w", err)
	}
	fileSize := stat.Size()
	fileMTime := stat.ModTime().Unix()

	// Parse options
	opt := &ResumableOptions{
		PartSize:    DefaultPartSize,
		Concurrency: DefaultConcurrency,
	}
	if len(opts) > 0 && opts[0] != nil {
		if opts[0].PartSize > 0 {
			opt.PartSize = opts[0].PartSize
		}
		if opts[0].Concurrency > 0 {
			opt.Concurrency = opts[0].Concurrency
		}
		if opts[0].CheckpointPath != "" {
			opt.CheckpointPath = opts[0].CheckpointPath
		}
		opt.Progress = opts[0].Progress
		opt.ContentType = opts[0].ContentType
	}
	if opt.CheckpointPath == "" {
		opt.CheckpointPath = localFilePath + ".cp"
	}
	if opt.ContentType == "" {
		opt.ContentType = detectContentType(localFilePath)
	}

	// For small files below PartSize, upload directly in one PUT request
	if fileSize <= opt.PartSize {
		res, err := s.Put(ctx, key, file, fileSize, opt.ContentType)
		if err == nil {
			_ = os.Remove(opt.CheckpointPath)
			if opt.Progress != nil {
				opt.Progress(fileSize, fileSize)
			}
		}
		return res, err
	}

	totalParts := int((fileSize + opt.PartSize - 1) / opt.PartSize)

	// Try loading checkpoint
	cp, err := loadCheckpoint(opt.CheckpointPath)
	var upload *MultipartUpload
	if err == nil && cp != nil && cp.FilePath == localFilePath && cp.FileSize == fileSize && cp.FileMTime == fileMTime && cp.PartSize == opt.PartSize {
		// Valid existing checkpoint found
		upload = &MultipartUpload{
			UploadID: cp.UploadID,
			Key:      cp.Key,
		}
	} else {
		// Start new multipart upload
		upload, err = s.InitiateMultipartUpload(ctx, key, opt.ContentType)
		if err != nil {
			return nil, fmt.Errorf("initiate multipart upload failed: %w", err)
		}
		cp = &Checkpoint{
			UploadID:   upload.UploadID,
			Key:        key,
			FilePath:   localFilePath,
			FileSize:   fileSize,
			FileMTime:  fileMTime,
			PartSize:   opt.PartSize,
			TotalParts: totalParts,
			Parts:      make(map[int]*Part),
		}
		_ = cp.save(opt.CheckpointPath)
	}

	// Calculate initial progress
	var uploadedBytes int64
	for _, p := range cp.Parts {
		uploadedBytes += p.Size
	}
	if opt.Progress != nil {
		opt.Progress(uploadedBytes, fileSize)
	}

	// Build list of parts remaining to upload
	type partTask struct {
		partNum  int
		offset   int64
		partSize int64
	}

	tasks := make([]partTask, 0, totalParts)
	for i := 1; i <= totalParts; i++ {
		if _, exists := cp.Parts[i]; exists {
			continue // Already uploaded
		}
		offset := int64(i-1) * opt.PartSize
		pSize := opt.PartSize
		if offset+pSize > fileSize {
			pSize = fileSize - offset
		}
		tasks = append(tasks, partTask{
			partNum:  i,
			offset:   offset,
			partSize: pSize,
		})
	}

	if len(tasks) > 0 {
		taskChan := make(chan partTask, len(tasks))
		for _, t := range tasks {
			taskChan <- t
		}
		close(taskChan)

		concurrency := opt.Concurrency
		if concurrency > len(tasks) {
			concurrency = len(tasks)
		}

		var wg sync.WaitGroup
		errChan := make(chan error, concurrency)

		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Open dedicated read handle per worker for concurrency safety
				workerFile, openErr := os.Open(localFilePath)
				if openErr != nil {
					select {
					case errChan <- openErr:
					default:
					}
					return
				}
				defer workerFile.Close()

				for task := range taskChan {
					select {
					case <-ctx.Done():
						select {
						case errChan <- ctx.Err():
						default:
						}
						return
					default:
					}

					sr := io.NewSectionReader(workerFile, task.offset, task.partSize)
					part, uploadErr := s.UploadPart(ctx, upload, task.partNum, sr, task.partSize)
					if uploadErr != nil {
						select {
						case errChan <- fmt.Errorf("upload part %d failed: %w", task.partNum, uploadErr):
						default:
						}
						return
					}

					cp.Mu.Lock()
					cp.Parts[task.partNum] = part
					cp.Mu.Unlock()
					_ = cp.save(opt.CheckpointPath)

					newUploaded := atomic.AddInt64(&uploadedBytes, task.partSize)
					if opt.Progress != nil {
						opt.Progress(newUploaded, fileSize)
					}
				}
			}()
		}

		wg.Wait()

		select {
		case err := <-errChan:
			return nil, err
		default:
		}
	}

	// Verify all parts are present
	if len(cp.Parts) != totalParts {
		return nil, fmt.Errorf("resumable upload incomplete: %d/%d parts uploaded", len(cp.Parts), totalParts)
	}

	// Sort parts in ascending order of PartNumber
	parts := make([]*Part, 0, totalParts)
	for _, p := range cp.Parts {
		parts = append(parts, p)
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	// Complete multipart upload
	res, err := s.CompleteMultipartUpload(ctx, upload, parts)
	if err != nil {
		return nil, fmt.Errorf("complete multipart upload failed: %w", err)
	}

	// Remove checkpoint file upon success
	_ = os.Remove(opt.CheckpointPath)
	return res, nil
}
