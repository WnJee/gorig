package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/xid"
)

type LocalStorage struct {
	rootPath string
	domain   string
}

func init() {
	RegisterDriver(Local, func(cfg *Config) (Storage, error) {
		root := "./.storage/uploads/"
		if cfg != nil && cfg.RootPath != "" {
			root = cfg.RootPath
		}
		domain := ""
		if cfg != nil && cfg.Domain != "" {
			domain = cfg.Domain
		}
		if err := os.MkdirAll(root, 0755); err != nil {
			return nil, fmt.Errorf("create local storage root dir failed: %w", err)
		}
		return &LocalStorage{
			rootPath: root,
			domain:   domain,
		}, nil
	})
}

func (l *LocalStorage) fullPath(key string) string {
	cleanKey := filepath.Clean("/" + strings.TrimPrefix(key, "/"))
	return filepath.Join(l.rootPath, cleanKey)
}

func (l *LocalStorage) chunkDir(uploadID string) string {
	return filepath.Join(l.rootPath, ".chunks", uploadID)
}

func (l *LocalStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (*PutResult, error) {
	dst := l.fullPath(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return nil, fmt.Errorf("create parent dir failed: %w", err)
	}

	tmpDst := dst + ".tmp." + xid.New().String()
	out, err := os.Create(tmpDst)
	if err != nil {
		return nil, fmt.Errorf("create local target file failed: %w", err)
	}
	defer func() {
		out.Close()
		_ = os.Remove(tmpDst)
	}()

	hash := md5.New()
	mw := io.MultiWriter(out, hash)

	written, err := io.Copy(mw, r)
	if err != nil {
		return nil, fmt.Errorf("write local file failed: %w", err)
	}
	out.Close()

	if err := os.Rename(tmpDst, dst); err != nil {
		return nil, fmt.Errorf("commit local file failed: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	url, _ := l.GetURL(ctx, key, 0)
	return &PutResult{
		Key:  key,
		ETag: etag,
		Size: written,
		URL:  url,
	}, nil
}

func (l *LocalStorage) PutBytes(ctx context.Context, key string, data []byte, contentType string) (*PutResult, error) {
	return PutBytesHelper(ctx, l, key, data, contentType)
}

func (l *LocalStorage) PutFile(ctx context.Context, key string, localFilePath string) (*PutResult, error) {
	return PutFileHelper(ctx, l, key, localFilePath)
}

func (l *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	dst := l.fullPath(key)
	f, err := os.Open(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, err
	}
	return f, nil
}

func (l *LocalStorage) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return GetBytesHelper(ctx, l, key)
}

func (l *LocalStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	cleanKey := strings.TrimPrefix(filepath.ToSlash(key), "/")
	if l.domain != "" {
		return strings.TrimRight(l.domain, "/") + "/" + cleanKey, nil
	}
	return "/" + cleanKey, nil
}

func (l *LocalStorage) Delete(ctx context.Context, key string) error {
	dst := l.fullPath(key)
	err := os.Remove(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (l *LocalStorage) DeleteMulti(ctx context.Context, keys []string) error {
	var errs []string
	for _, key := range keys {
		if err := l.Delete(ctx, key); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("delete multi failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (l *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	dst := l.fullPath(key)
	_, err := os.Stat(dst)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (l *LocalStorage) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	dst := l.fullPath(key)
	st, err := os.Stat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, err
	}
	return &ObjectInfo{
		Key:          key,
		Size:         st.Size(),
		LastModified: st.ModTime(),
		ContentType:  detectContentType(dst),
	}, nil
}

func (l *LocalStorage) InitiateMultipartUpload(ctx context.Context, key string, contentType string) (*MultipartUpload, error) {
	uploadID := xid.New().String()
	chunkPath := l.chunkDir(uploadID)
	if err := os.MkdirAll(chunkPath, 0755); err != nil {
		return nil, fmt.Errorf("create chunk directory failed: %w", err)
	}
	return &MultipartUpload{
		UploadID: uploadID,
		Key:      key,
		Bucket:   "local",
	}, nil
}

func (l *LocalStorage) UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int, r io.Reader, partSize int64) (*Part, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}
	chunkPath := l.chunkDir(upload.UploadID)
	partFile := filepath.Join(chunkPath, fmt.Sprintf("part_%05d", partNumber))

	out, err := os.Create(partFile)
	if err != nil {
		return nil, fmt.Errorf("create part file failed: %w", err)
	}
	defer out.Close()

	hash := md5.New()
	mw := io.MultiWriter(out, hash)

	written, err := io.Copy(mw, r)
	if err != nil {
		return nil, fmt.Errorf("write part file failed: %w", err)
	}

	return &Part{
		PartNumber: partNumber,
		ETag:       hex.EncodeToString(hash.Sum(nil)),
		Size:       written,
	}, nil
}

func (l *LocalStorage) CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload, parts []*Part) (*PutResult, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}
	chunkPath := l.chunkDir(upload.UploadID)
	defer os.RemoveAll(chunkPath)

	dst := l.fullPath(upload.Key)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return nil, fmt.Errorf("create parent dir failed: %w", err)
	}

	tmpDst := dst + ".tmp." + upload.UploadID
	out, err := os.Create(tmpDst)
	if err != nil {
		return nil, fmt.Errorf("create merge destination file failed: %w", err)
	}
	defer func() {
		out.Close()
		_ = os.Remove(tmpDst)
	}()

	// Sort parts in order
	sortedParts := append([]*Part(nil), parts...)
	sort.Slice(sortedParts, func(i, j int) bool {
		return sortedParts[i].PartNumber < sortedParts[j].PartNumber
	})

	hash := md5.New()
	mw := io.MultiWriter(out, hash)

	var totalSize int64
	for _, p := range sortedParts {
		partFile := filepath.Join(chunkPath, fmt.Sprintf("part_%05d", p.PartNumber))
		pf, err := os.Open(partFile)
		if err != nil {
			return nil, fmt.Errorf("open part %d file failed: %w", p.PartNumber, err)
		}
		n, err := io.Copy(mw, pf)
		pf.Close()
		if err != nil {
			return nil, fmt.Errorf("merge part %d failed: %w", p.PartNumber, err)
		}
		totalSize += n
	}
	out.Close()

	if err := os.Rename(tmpDst, dst); err != nil {
		return nil, fmt.Errorf("rename merged file failed: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	url, _ := l.GetURL(ctx, upload.Key, 0)
	return &PutResult{
		Key:  upload.Key,
		ETag: etag,
		Size: totalSize,
		URL:  url,
	}, nil
}

func (l *LocalStorage) AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error {
	if upload == nil || upload.UploadID == "" {
		return nil
	}
	return os.RemoveAll(l.chunkDir(upload.UploadID))
}

func (l *LocalStorage) ListParts(ctx context.Context, upload *MultipartUpload) ([]*Part, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}
	chunkPath := l.chunkDir(upload.UploadID)
	entries, err := os.ReadDir(chunkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var parts []*Part
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "part_") {
			continue
		}
		var partNum int
		_, err := fmt.Sscanf(entry.Name(), "part_%d", &partNum)
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		parts = append(parts, &Part{
			PartNumber: partNum,
			Size:       info.Size(),
		})
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	return parts, nil
}

func (l *LocalStorage) UploadFileResumable(ctx context.Context, key string, localFilePath string, opts ...*ResumableOptions) (*PutResult, error) {
	return UploadFileResumableGeneric(ctx, l, key, localFilePath, opts...)
}
