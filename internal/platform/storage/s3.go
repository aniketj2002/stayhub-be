package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stayhub/backend/internal/config"
)

const localMediaDir = "data/media"
const localMediaRoute = "/api/v1/media/file"

// Storage provides AWS S3 Signature V4 presigned upload URLs, server-side S3 forwarding,
// local file persistence, and public read URLs for media.
type Storage struct {
	endpoint      string
	publicBaseURL string
	region        string
	bucket        string
	accessKey     string
	secretKey     string
	sessionToken  string
	usePathStyle  bool
	httpClient    *http.Client
}

func NewStorage(cfg *config.Config) *Storage {
	return &Storage{
		endpoint:      strings.TrimRight(cfg.S3Endpoint, "/"),
		publicBaseURL: strings.TrimRight(cfg.S3PublicBaseURL, "/"),
		region:        cfg.S3Region,
		bucket:        cfg.S3Bucket,
		accessKey:     cfg.S3AccessKey,
		secretKey:     cfg.S3SecretKey,
		sessionToken:  cfg.S3SessionToken,
		usePathStyle:  cfg.S3UsePathStyle,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}
}

// HasS3Credentials returns true if real AWS S3 credentials are configured.
func (s *Storage) HasS3Credentials() bool {
	return s.accessKey != "" && s.secretKey != "" && s.accessKey != "minioadmin"
}

// PublicURL returns the public read URL for a given S3 object key.
// Local development stores bytes on disk and serves them from the API.
// The MinIO placeholder credentials do not publish a reachable bucket.
func (s *Storage) PublicURL(s3Key string) string {
	if strings.HasPrefix(s3Key, "https://") || strings.HasPrefix(s3Key, "http://") {
		return s3Key
	}
	cleanKey := strings.TrimLeft(s3Key, "/")
	if !s.HasS3Credentials() {
		return localMediaRoute + "/" + cleanKey
	}
	return fmt.Sprintf("%s/%s", s.publicBaseURL, cleanKey)
}

// ResolveLocalPath safely maps an S3 object key to a local file path inside ./data/media.
// Reads check both the process working directory and the repository root, because the API
// is started from either stayhub/ or stayhub/backend/.
func (s *Storage) ResolveLocalPath(s3Key string) (string, error) {
	cleanKey := filepath.Clean(strings.TrimLeft(s3Key, "/"))
	if cleanKey == "." || cleanKey == ".." || strings.HasPrefix(cleanKey, "..") {
		return "", errors.New("invalid media key path")
	}
	roots, err := localMediaRoots()
	if err != nil {
		return "", err
	}
	var fallback string
	for i, root := range roots {
		baseAbs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		targetAbs := filepath.Join(baseAbs, cleanKey)
		if !strings.HasPrefix(targetAbs, baseAbs+string(filepath.Separator)) {
			return "", errors.New("media path escapes storage root")
		}
		if i == 0 {
			fallback = targetAbs
		}
		if _, statErr := os.Stat(targetAbs); statErr == nil {
			return targetAbs, nil
		}
	}
	if fallback == "" {
		return "", errors.New("media storage root is unavailable")
	}
	return fallback, nil
}

func localMediaRoots() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	roots := []string{filepath.Join(cwd, localMediaDir)}
	if filepath.Base(cwd) == "backend" {
		roots = append(roots, filepath.Join(filepath.Dir(cwd), localMediaDir))
	} else {
		roots = append(roots, filepath.Join(cwd, "backend", localMediaDir))
	}
	return roots, nil
}

// SaveObject persists the uploaded bytes locally (for fast serving via /api/v1/media/file/*)
// and, if AWS S3 credentials are configured, also uploads the object to AWS S3.
func (s *Storage) SaveObject(ctx context.Context, s3Key, contentType string, data []byte, now time.Time) error {
	localPath, err := s.ResolveLocalPath(s3Key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("mkdir media dir: %w", err)
	}
	if err := os.WriteFile(localPath, data, 0o644); err != nil {
		return fmt.Errorf("write local media file: %w", err)
	}

	if s.HasS3Credentials() {
		presignedURL, err := s.PresignPut(s3Key, contentType, 15*time.Minute, now)
		if err != nil {
			return fmt.Errorf("presign s3 put: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("create s3 put request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.ContentLength = int64(len(data))

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("upload to s3: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("s3 put returned status %d", resp.StatusCode)
		}
	}

	return nil
}

// PresignPut generates an AWS Signature V4 presigned PUT URL valid for expires duration.
func (s *Storage) PresignPut(s3Key, contentType string, expires time.Duration, now time.Time) (string, error) {
	u, err := url.Parse(s.endpoint)
	if err != nil {
		return "", fmt.Errorf("parse s3 endpoint: %w", err)
	}

	cleanKey := strings.TrimLeft(s3Key, "/")
	var canonicalURI string
	host := u.Host
	if s.usePathStyle {
		canonicalURI = fmt.Sprintf("/%s/%s", s.bucket, cleanKey)
	} else {
		if strings.HasPrefix(host, s.bucket+".") {
			canonicalURI = fmt.Sprintf("/%s", cleanKey)
		} else {
			host = fmt.Sprintf("%s.%s", s.bucket, u.Host)
			canonicalURI = fmt.Sprintf("/%s", cleanKey)
		}
	}

	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", s.accessKey, credentialScope))
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", int(expires.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")
	if s.sessionToken != "" {
		q.Set("X-Amz-Security-Token", s.sessionToken)
	}

	canonicalQueryString := q.Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	payloadHash := "UNSIGNED-PAYLOAD"

	canonicalRequest := strings.Join([]string{
		"PUT",
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		"host",
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(s.secretKey, dateStamp, s.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	presignedURL := fmt.Sprintf("%s://%s%s?%s&X-Amz-Signature=%s",
		u.Scheme, host, canonicalURI, canonicalQueryString, signature)

	return presignedURL, nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
