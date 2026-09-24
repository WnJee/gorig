package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

type S3Storage struct {
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	bucket          string
	region          string
	useSSL          bool
	domain          string
	pathStyle       bool
	client          *http.Client
}

func init() {
	s3Factory := func(cfg *Config) (Storage, error) {
		if cfg == nil || cfg.Endpoint == "" || cfg.Bucket == "" {
			return nil, fmt.Errorf("s3/minio storage requires endpoint and bucket")
		}
		endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "http://"), "https://")
		region := cfg.Region
		if region == "" {
			region = "us-east-1"
		}
		return &S3Storage{
			endpoint:        endpoint,
			accessKeyID:     cfg.AccessKeyID,
			accessKeySecret: cfg.AccessKeySecret,
			bucket:          cfg.Bucket,
			region:          region,
			useSSL:          cfg.UseSSL,
			domain:          cfg.Domain,
			pathStyle:       cfg.PathStyle,
			client:          &http.Client{Timeout: 60 * time.Second},
		}, nil
	}

	RegisterDriver(MinIO, s3Factory)
	RegisterDriver(S3, s3Factory)
}

func (s *S3Storage) getBaseURL() string {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, s.endpoint)
}

func (s *S3Storage) getObjectURI(key string) string {
	cleanKey := "/" + strings.TrimPrefix(key, "/")
	if s.pathStyle {
		return "/" + s.bucket + cleanKey
	}
	return cleanKey
}

func (s *S3Storage) getHost() string {
	if s.pathStyle {
		return s.endpoint
	}
	return s.bucket + "." + s.endpoint
}

// sha256Hex returns the hex-encoded SHA256 of data
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// hmacSHA256 returns HMAC-SHA256 signature bytes
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// getSignatureKey derives the SigV4 signing key
func getSignatureKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

// signRequest signs the HTTP request using AWS SigV4
func (s *S3Storage) signRequest(req *http.Request, payloadHash string, t time.Time) {
	dateStamp := t.UTC().Format("20060102")
	amzDate := t.UTC().Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Host", s.getHost())

	// Canonical headers
	headersToSign := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if ct := req.Header.Get("Content-Type"); ct != "" {
		headersToSign = append(headersToSign, "content-type")
	}
	sort.Strings(headersToSign)

	var canonicalHeaders strings.Builder
	for _, h := range headersToSign {
		val := strings.TrimSpace(req.Header.Get(h))
		if h == "host" {
			val = s.getHost()
		}
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", h, val))
	}
	signedHeaders := strings.Join(headersToSign, ";")

	// Canonical query string
	query := req.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canonicalQuery strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonicalQuery.WriteString("&")
		}
		canonicalQuery.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(query.Get(k)))
	}

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery.String(),
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := getSignatureKey(s.accessKeySecret, dateStamp, s.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func (s *S3Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (*PutResult, error) {
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

	payloadHash := sha256Hex(data)
	uri := s.getObjectURI(key)
	fullURL := s.getBaseURL() + uri

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))

	s.signRequest(req, payloadHash, time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 put failed with status %d: %s", resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	url, _ := s.GetURL(ctx, key, 0)
	return &PutResult{
		Key:  key,
		ETag: etag,
		Size: int64(len(data)),
		URL:  url,
	}, nil
}

func (s *S3Storage) PutBytes(ctx context.Context, key string, data []byte, contentType string) (*PutResult, error) {
	return PutBytesHelper(ctx, s, key, data, contentType)
}

func (s *S3Storage) PutFile(ctx context.Context, key string, localFilePath string) (*PutResult, error) {
	return PutFileHelper(ctx, s, key, localFilePath)
}

func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	uri := s.getObjectURI(key)
	fullURL := s.getBaseURL() + uri

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	s.signRequest(req, sha256Hex([]byte("")), time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("s3 get failed with status: %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (s *S3Storage) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return GetBytesHelper(ctx, s, key)
}

func (s *S3Storage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if s.domain != "" {
		return strings.TrimRight(s.domain, "/") + "/" + strings.TrimPrefix(key, "/"), nil
	}

	if expires <= 0 {
		return s.getBaseURL() + s.getObjectURI(key), nil
	}

	// Presigned URL generation using SigV4 Query Parameters
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)

	uri := s.getObjectURI(key)
	q := make(url.Values)
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", s.accessKeyID+"/"+credentialScope)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(int(expires.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")

	canonicalRequest := strings.Join([]string{
		http.MethodGet,
		uri,
		q.Encode(),
		"host:" + s.getHost() + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := getSignatureKey(s.accessKeySecret, dateStamp, s.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	q.Set("X-Amz-Signature", signature)
	return fmt.Sprintf("%s%s?%s", s.getBaseURL(), uri, q.Encode()), nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	uri := s.getObjectURI(key)
	fullURL := s.getBaseURL() + uri

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, nil)
	if err != nil {
		return err
	}
	s.signRequest(req, sha256Hex([]byte("")), time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("s3 delete failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (s *S3Storage) DeleteMulti(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	info, err := s.Stat(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return info != nil, nil
}

func (s *S3Storage) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	uri := s.getObjectURI(key)
	fullURL := s.getBaseURL() + uri

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fullURL, nil)
	if err != nil {
		return nil, err
	}
	s.signRequest(req, sha256Hex([]byte("")), time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("s3 stat failed with status: %d", resp.StatusCode)
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

type initiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

func (s *S3Storage) InitiateMultipartUpload(ctx context.Context, key string, contentType string) (*MultipartUpload, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	uri := s.getObjectURI(key)
	fullURL := s.getBaseURL() + uri + "?uploads="

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	s.signRequest(req, sha256Hex([]byte("")), time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 initiate multipart upload failed (%d): %s", resp.StatusCode, string(body))
	}

	var res initiateMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode initiate multipart result failed: %w", err)
	}

	return &MultipartUpload{
		UploadID: res.UploadId,
		Key:      key,
		Bucket:   s.bucket,
	}, nil
}

func (s *S3Storage) UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int, r io.Reader, partSize int64) (*Part, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}

	data, err := io.ReadAll(io.LimitReader(r, partSize))
	if err != nil {
		return nil, fmt.Errorf("read part data failed: %w", err)
	}

	uri := s.getObjectURI(upload.Key)
	q := url.Values{}
	q.Set("partNumber", strconv.Itoa(partNumber))
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s%s?%s", s.getBaseURL(), uri, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))

	payloadHash := sha256Hex(data)
	s.signRequest(req, payloadHash, time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 upload part %d failed (%d): %s", partNumber, resp.StatusCode, string(body))
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	return &Part{
		PartNumber: partNumber,
		ETag:       etag,
		Size:       int64(len(data)),
	}, nil
}

type completeMultipartUploadRequest struct {
	XMLName xml.Name                      `xml:"CompleteMultipartUpload"`
	Parts   []completeMultipartUploadPart `xml:"Part"`
}

type completeMultipartUploadPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type completeMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

func (s *S3Storage) CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload, parts []*Part) (*PutResult, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}

	reqPayload := completeMultipartUploadRequest{
		Parts: make([]completeMultipartUploadPart, len(parts)),
	}
	var totalSize int64
	for i, p := range parts {
		reqPayload.Parts[i] = completeMultipartUploadPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
		totalSize += p.Size
	}

	xmlData, err := xml.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	uri := s.getObjectURI(upload.Key)
	q := url.Values{}
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s%s?%s", s.getBaseURL(), uri, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(xmlData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Content-Length", strconv.Itoa(len(xmlData)))

	payloadHash := sha256Hex(xmlData)
	s.signRequest(req, payloadHash, time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 complete multipart upload failed (%d): %s", resp.StatusCode, string(body))
	}

	var res completeMultipartUploadResult
	_ = xml.NewDecoder(resp.Body).Decode(&res)

	etag := strings.Trim(res.ETag, "\"")
	url, _ := s.GetURL(ctx, upload.Key, 0)
	return &PutResult{
		Key:  upload.Key,
		ETag: etag,
		Size: totalSize,
		URL:  url,
	}, nil
}

func (s *S3Storage) AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error {
	if upload == nil || upload.UploadID == "" {
		return nil
	}
	uri := s.getObjectURI(upload.Key)
	q := url.Values{}
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s%s?%s", s.getBaseURL(), uri, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, nil)
	if err != nil {
		return err
	}
	s.signRequest(req, sha256Hex([]byte("")), time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("s3 abort multipart failed with status: %d", resp.StatusCode)
	}
	return nil
}

type listPartsResult struct {
	XMLName xml.Name        `xml:"ListPartsResult"`
	Parts   []listPartsPart `xml:"Part"`
}

type listPartsPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
	Size       int64  `xml:"Size"`
}

func (s *S3Storage) ListParts(ctx context.Context, upload *MultipartUpload) ([]*Part, error) {
	if upload == nil || upload.UploadID == "" {
		return nil, fmt.Errorf("invalid multipart upload session")
	}
	uri := s.getObjectURI(upload.Key)
	q := url.Values{}
	q.Set("uploadId", upload.UploadID)
	fullURL := fmt.Sprintf("%s%s?%s", s.getBaseURL(), uri, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	s.signRequest(req, sha256Hex([]byte("")), time.Now())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("s3 list parts failed: %d", resp.StatusCode)
	}

	var res listPartsResult
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

func (s *S3Storage) UploadFileResumable(ctx context.Context, key string, localFilePath string, opts ...*ResumableOptions) (*PutResult, error) {
	return UploadFileResumableGeneric(ctx, s, key, localFilePath, opts...)
}
