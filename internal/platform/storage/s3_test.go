package storage

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stayhub/backend/internal/config"
)

func TestAWSS3PresignPutAndPublicURL(t *testing.T) {
	cfg := &config.Config{
		S3Endpoint:      "https://s3.eu-west-1.amazonaws.com",
		S3PublicBaseURL: "https://stayhub-media.s3.eu-west-1.amazonaws.com",
		S3Region:        "eu-west-1",
		S3Bucket:        "stayhub-media",
		S3AccessKey:     "AKIAIOSFODNN7EXAMPLE",
		S3SecretKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		S3SessionToken:  "AQoDYXdzEJr...",
		S3UsePathStyle:  false,
	}

	store := NewStorage(cfg)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

	s3Key := "uploads/listing_photo/user-1/photo-1.jpg"
	presignedURL, err := store.PresignPut(s3Key, "image/jpeg", 15*time.Minute, now)
	if err != nil {
		t.Fatalf("unexpected error generating AWS S3 presigned URL: %v", err)
	}

	u, err := url.Parse(presignedURL)
	if err != nil {
		t.Fatalf("invalid URL generated: %v", err)
	}
	if u.Host != "stayhub-media.s3.eu-west-1.amazonaws.com" {
		t.Fatalf("expected virtual-hosted S3 host, got %s", u.Host)
	}
	if u.Path != "/uploads/listing_photo/user-1/photo-1.jpg" {
		t.Fatalf("unexpected S3 path: %s", u.Path)
	}
	q := u.Query()
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" || q.Get("X-Amz-Signature") == "" || q.Get("X-Amz-Security-Token") == "" {
		t.Fatalf("missing expected AWS SigV4 query parameters: %v", q)
	}

	pubURL := store.PublicURL(s3Key)
	if !strings.HasPrefix(pubURL, "https://stayhub-media.s3.eu-west-1.amazonaws.com/uploads/") {
		t.Fatalf("unexpected public URL: %s", pubURL)
	}
}

func TestLocalPublicURLUsesAPIFileRoute(t *testing.T) {
	store := NewStorage(&config.Config{
		S3Endpoint:      "http://localhost:9000",
		S3PublicBaseURL: "http://localhost:9000/stayhub-media",
		S3Region:        "eu-west-1",
		S3Bucket:        "stayhub-media",
		S3AccessKey:     "minioadmin",
		S3SecretKey:     "minioadmin",
		S3UsePathStyle:  true,
	})
	got := store.PublicURL("uploads/listing_photo/user/photo.jpg")
	want := "/api/v1/media/file/uploads/listing_photo/user/photo.jpg"
	if got != want {
		t.Fatalf("public URL = %s, want %s", got, want)
	}
	if store.PublicURL("https://images.example/photo.jpg") != "https://images.example/photo.jpg" {
		t.Fatal("absolute photo URLs should be returned unchanged")
	}
}
