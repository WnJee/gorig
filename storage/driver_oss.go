package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type OSSStorage struct {
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	bucket          string
	useSSL          bool
	domain          string
	client          *http.Client
}

func init() {
	RegisterDriver(OSS, func(cfg *Config) (Storage, error) {
		if cfg == nil || cfg.Endpoint == "" || cfg.Bucket == "" {
			return nil, fmt.Errorf("aliyun oss storage requires endpoint and bucket")
		}
		endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "http://"), "https://")
		return &OSSStorage{
			endpoint:        endpoint,
			accessKeyID:     cfg.AccessKeyID,
			accessKeySecret: cfg.AccessKeySecret,
			bucket:          cfg.Bucket,
			useSSL:          cfg.UseSSL,
			domain:          cfg.Domain,
			client:          &http.Client{Timeout: 60 * time.Second},
		}, nil
	})
}

func (o *OSSStorage) getHost() string {
	if strings.HasPrefix(o.endpoint, o.bucket+".") {
		return o.endpoint
	}
	return o.bucket + "." + o.endpoint
}

func (o *OSSStorage) getBaseURL() string {
	scheme := "http"
	if o.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, o.getHost())
}

func (o *OSSStorage) getCanonicalResource(key string, query url.Values) string {
	cleanKey := strings.TrimPrefix(key, "/")
	res := "/" + o.bucket + "/" + cleanKey

	// Specific subresources to include in canonical resource
	subresources := []string{"uploads", "uploadId", "partNumber"}
	var subParams []string
	for _, sub := range subresources {
		if query.Has(sub) {
			val := query.Get(sub)
			if val != "" {
				subParams = append(subParams, sub+"="+val)
			} else {
				subParams = append(subParams, sub)
			}
		}
	}
	if len(subParams) > 0 {
		res += "?" + strings.Join(subParams, "&")
	}
	return res
}

func (o *OSSStorage) signRequest(req *http.Request, key string, t time.Time) {
	dateStr := t.UTC().Format(http.TimeFormat)
	req.Header.Set("Date", dateStr)
	req.Header.Set("Host", o.getHost())

	// Build CanonicalizedOSSHeaders
	var ossHeaders []string
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-oss-") {
			ossHeaders = append(ossHeaders, lk+":"+strings.Join(v, ","))
		}
	}
	sort.Strings(ossHeaders)
	canonicalOSSHeaders := ""
	if len(ossHeaders) > 0 {
		canonicalOSSHeaders = strings.Join(ossHeaders, "\n") + "\n"
	}

	canonicalResource := o.getCanonicalResource(key, req.URL.Query())

	stringToSign := strings.Join([]string{
		req.Method,
		req.Header.Get("Content-MD5"),
		req.Header.Get("Content-Type"),
		dateStr,
		canonicalOSSHeaders + canonicalResource,
	}, "\n")

	h := hmac.New(sha1.New, []byte(o.accessKeySecret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req.Header.Set("Authorization", fmt.Sprintf("OSS %s:%s", o.accessKeyID, signature))
}

func (o *OSSStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (*PutResult, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var data []byte
	var err error
	if size > 0 {
		data, err = io.ReadAll(io.LimitReader(r, size))
	} else {
		data, err = io.ReadAll(r)
	}
	if err != nil {
		return nil, fmt.Errorf("read put data failed: %w", err)
	}

	cleanKey := strings.TrimPrefix(key, "/")
	fullURL := o.getBaseURL() + "/" + cleanKey

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	o.signRequest(req, key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oss put failed (%d): %s", resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	url, _ := o.GetURL(ctx, key, 0)
	return &PutResult{
		Key:  key,
		ETag: etag,
		Size: int64(len(data)),
		URL:  url,
	}, nil
}

func (o *OSSStorage) PutBytes(ctx context.Context, key string, data []byte, contentType string) (*PutResult, error) {
	return PutBytesHelper(ctx, o, key, data, contentType)
}

func (o *OSSStorage) PutFile(ctx context.Context, key string, localFilePath string) (*PutResult, error) {
	return PutFileHelper(ctx, o, key, localFilePath)
}

func (o *OSSStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	fullURL := o.getBaseURL() + "/" + cleanKey

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	o.signRequest(req, key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("oss get failed (%d)", resp.StatusCode)
	}
	return resp.Body, nil
}

func (o *OSSStorage) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return GetBytesHelper(ctx, o, key)
}

func (o *OSSStorage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	if o.domain != "" {
		return strings.TrimRight(o.domain, "/") + "/" + cleanKey, nil
	}

	if expires <= 0 {
		return o.getBaseURL() + "/" + cleanKey, nil
	}

	// Presigned URL generation for OSS
	expireTimestamp := time.Now().Add(expires).Unix()
	expiresStr := strconv.FormatInt(expireTimestamp, 10)

	canonicalResource := "/" + o.bucket + "/" + cleanKey
	stringToSign := fmt.Sprintf("%s\n\n\n%s\n%s", http.MethodGet, expiresStr, canonicalResource)

	h := hmac.New(sha1.New, []byte(o.accessKeySecret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	q := make(url.Values)
	q.Set("OSSAccessKeyId", o.accessKeyID)
	q.Set("Expires", expiresStr)
	q.Set("Signature", signature)

	return fmt.Sprintf("%s/%s?%s", o.getBaseURL(), cleanKey, q.Encode()), nil
}

func (o *OSSStorage) Delete(ctx context.Context, key string) error {
	cleanKey := strings.TrimPrefix(key, "/")
	fullURL := o.getBaseURL() + "/" + cleanKey

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, nil)
	if err != nil {
		return err
	}
	o.signRequest(req, key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("oss delete failed: %d", resp.StatusCode)
	}
	return nil
}

func (o *OSSStorage) DeleteMulti(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := o.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (o *OSSStorage) Exists(ctx context.Context, key string) (bool, error) {
	info, err := o.Stat(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return info != nil, nil
}

func (o *OSSStorage) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	cleanKey := strings.TrimPrefix(key, "/")
	fullURL := o.getBaseURL() + "/" + cleanKey

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fullURL, nil)
	if err != nil {
		return nil, err
	}
	o.signRequest(req, key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oss stat failed (%d)", resp.StatusCode)
	}

	size, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	lastMod, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	return &ObjectInfo{
		Key:          key,
		Size:         size,
		ETag:         strings.Trim(resp.Header.Get("ETag"), "\""),
		LastModified: lastMod,
		ContentType:  resp.Header.Get("Content-Type"),
	}, nil
}

// Multipart Upload Methods

type ossInitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

func (o *OSSStorage) InitiateMultipartUpload(ctx context.Context, key string, contentType string) (*MultipartUpload, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	cleanKey := strings.TrimPrefix(key, "/")
	fullURL := o.getBaseURL() + "/" + cleanKey + "?uploads"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	o.signRequest(req, key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oss initiate multipart failed (%d): %s", resp.StatusCode, string(body))
	}

	var res ossInitiateMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &MultipartUpload{
		UploadID: res.UploadId,
		Key:      key,
		Bucket:   o.bucket,
	}, nil
}

func (o *OSSStorage) UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int, r io.Reader, partSize int64) (*Part, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}

	data, err := io.ReadAll(io.LimitReader(r, partSize))
	if err != nil {
		return nil, fmt.Errorf("read part data failed: %w", err)
	}

	cleanKey := strings.TrimPrefix(upload.Key, "/")
	q := url.Values{}
	q.Set("partNumber", strconv.Itoa(partNumber))
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s/%s?%s", o.getBaseURL(), cleanKey, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))
	o.signRequest(req, upload.Key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oss upload part %d failed (%d): %s", partNumber, resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	return &Part{
		PartNumber: partNumber,
		ETag:       etag,
		Size:       int64(len(data)),
	}, nil
}

type ossCompleteMultipartUploadRequest struct {
	XMLName xml.Name                         `xml:"CompleteMultipartUpload"`
	Parts   []ossCompleteMultipartUploadPart `xml:"Part"`
}

type ossCompleteMultipartUploadPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

func (o *OSSStorage) CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload, parts []*Part) (*PutResult, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}

	reqPayload := ossCompleteMultipartUploadRequest{
		Parts: make([]ossCompleteMultipartUploadPart, len(parts)),
	}
	var totalSize int64
	for i, p := range parts {
		reqPayload.Parts[i] = ossCompleteMultipartUploadPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
		totalSize += p.Size
	}

	xmlData, err := xml.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	cleanKey := strings.TrimPrefix(upload.Key, "/")
	q := url.Values{}
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s/%s?%s", o.getBaseURL(), cleanKey, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(xmlData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Content-Length", strconv.Itoa(len(xmlData)))
	o.signRequest(req, upload.Key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oss complete multipart failed (%d): %s", resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	url, _ := o.GetURL(ctx, upload.Key, 0)
	return &PutResult{
		Key:  upload.Key,
		ETag: etag,
		Size: totalSize,
		URL:  url,
	}, nil
}

func (o *OSSStorage) AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error {
	if upload == nil || upload.UploadID == "" {
		return nil
	}
	cleanKey := strings.TrimPrefix(upload.Key, "/")
	q := url.Values{}
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s/%s?%s", o.getBaseURL(), cleanKey, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, nil)
	if err != nil {
		return err
	}
	o.signRequest(req, upload.Key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oss abort multipart failed: %d", resp.StatusCode)
	}
	return nil
}

type ossListPartsResult struct {
	XMLName xml.Name           `xml:"ListPartsResult"`
	Parts   []ossListPartsPart `xml:"Part"`
}

type ossListPartsPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
	Size       int64  `xml:"Size"`
}

func (o *OSSStorage) ListParts(ctx context.Context, upload *MultipartUpload) ([]*Part, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}
	cleanKey := strings.TrimPrefix(upload.Key, "/")
	q := url.Values{}
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s/%s?%s", o.getBaseURL(), cleanKey, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	o.signRequest(req, upload.Key, time.Now())

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oss list parts failed: %d", resp.StatusCode)
	}

	var res ossListPartsResult
	if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	parts := make([]*Part, len(res.Parts))
	for i, p := range res.Parts {
		parts[i] = &Part{
			PartNumber: p.PartNumber,
			ETag:       strings.Trim(p.ETag, "\""),
			Size:       p.Size,
		}
	}
	return parts, nil
}

func (o *OSSStorage) UploadFileResumable(ctx context.Context, key string, localFilePath string, opts ...*ResumableOptions) (*PutResult, error) {
	return UploadFileResumableGeneric(ctx, o, key, localFilePath, opts...)
}
