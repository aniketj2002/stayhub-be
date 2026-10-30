package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/domain/statemachine"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/storage"
)

const presignUploadTTL = 15 * time.Minute

var contentTypeExtMap = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/avif": ".avif",
	"image/gif":  ".gif",
}

type MediaService struct {
	db        *db.DB
	mediaRepo interfaces.MediaRepository
	storage   *storage.Storage
	audit     interfaces.AuditService
	clock     clock.Clock
}

func NewMediaService(
	database *db.DB,
	mediaRepo interfaces.MediaRepository,
	store *storage.Storage,
	audit interfaces.AuditService,
	clk clock.Clock,
) interfaces.MediaService {
	return &MediaService{
		db:        database,
		mediaRepo: mediaRepo,
		storage:   store,
		audit:     audit,
		clock:     clk,
	}
}

func (s *MediaService) PresignUpload(ctx context.Context, actor models.AuthActor, in dtos.PresignMediaInput) (*dtos.PresignMediaResponse, error) {
	mediaID := uuid.New()
	contentType := strings.ToLower(strings.TrimSpace(in.ContentType))
	ext := contentTypeExtMap[contentType]
	if ext == "" {
		ext = ".bin"
	}

	purpose := strings.TrimSpace(in.Purpose)
	if purpose == "" {
		purpose = "listing_photo"
	}

	s3Key := fmt.Sprintf("uploads/%s/%s/%s%s", purpose, actor.SubjectID.String(), mediaID.String(), ext)
	now := s.clock.Now()

	// Route the browser PUT through /api/v1/media/:id/upload so browser-to-S3 CORS never blocks uploads,
	// while the backend forwards the object to AWS S3 (when AWS credentials are set) and caches locally.
	uploadURL := fmt.Sprintf("/api/v1/media/%s/upload", mediaID.String())

	var created *models.Media
	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		m, err := s.mediaRepo.CreatePending(txCtx, dtos.CreateMediaParams{
			ID:          mediaID,
			S3Key:       s3Key,
			ContentType: contentType,
			SizeBytes:   in.SizeBytes,
			Width:       in.Width,
			Height:      in.Height,
			UploadedBy:  actor.SubjectID,
		})
		if err != nil {
			return err
		}
		created = m

		actorID := actor.SubjectID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:  actor.DefaultActorType(),
			ActorID:    &actorID,
			Action:     "media.presign",
			ObjectType: "media",
			ObjectID:   m.ID,
			AfterState: map[string]any{
				"s3_key":       m.S3Key,
				"content_type": m.ContentType,
				"size_bytes":   m.SizeBytes,
				"status":       m.Status,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	publicURL := s.storage.PublicURL(created.S3Key)
	return &dtos.PresignMediaResponse{
		Media:     dtos.ToMediaResponse(*created, publicURL),
		UploadURL: uploadURL,
		Method:    "PUT",
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		ExpiresAt: now.UTC().Add(presignUploadTTL),
	}, nil
}

func (s *MediaService) UploadContent(ctx context.Context, mediaID uuid.UUID, contentType string, data []byte) error {
	existing, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return apperrors.ErrValidation
	}
	ct := existing.ContentType
	if trimmed := strings.TrimSpace(contentType); trimmed != "" {
		ct = trimmed
	}
	return s.storage.SaveObject(ctx, existing.S3Key, ct, data, s.clock.Now())
}

func (s *MediaService) ConfirmUpload(ctx context.Context, actor models.AuthActor, mediaID uuid.UUID, in dtos.ConfirmMediaInput) (*dtos.MediaResponse, error) {
	var confirmed *models.Media

	err := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		existing, err := s.mediaRepo.FindByID(txCtx, mediaID)
		if err != nil {
			return err
		}

		if existing.UploadedBy != actor.SubjectID && actor.SubjectType != "admin" {
			return apperrors.ErrMediaOwnership
		}

		if err := statemachine.ValidateMediaTransition(existing.Status, statemachine.MediaStatusUploaded); err != nil {
			return err
		}
		if existing.Status == statemachine.MediaStatusUploaded {
			confirmed = existing
			return nil
		}

		updated, err := s.mediaRepo.ConfirmUpload(txCtx, mediaID, in.Width, in.Height)
		if err != nil {
			return err
		}
		confirmed = updated

		actorID := actor.SubjectID
		return s.audit.Record(txCtx, dtos.AuditEntryInput{
			ActorType:   actor.DefaultActorType(),
			ActorID:     &actorID,
			Action:      "media.confirm",
			ObjectType:  "media",
			ObjectID:    updated.ID,
			BeforeState: map[string]any{"status": existing.Status},
			AfterState: map[string]any{
				"status": updated.Status,
				"width":  updated.Width,
				"height": updated.Height,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	resp := dtos.ToMediaResponse(*confirmed, s.storage.PublicURL(confirmed.S3Key))
	return &resp, nil
}

func (s *MediaService) GetByID(ctx context.Context, mediaID uuid.UUID) (*dtos.MediaResponse, error) {
	m, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	resp := dtos.ToMediaResponse(*m, s.storage.PublicURL(m.S3Key))
	return &resp, nil
}

func (s *MediaService) ResolveLocalFilePath(s3Key string) (string, error) {
	return s.storage.ResolveLocalPath(s3Key)
}

func (s *MediaService) PublicURL(s3Key string) string {
	return s.storage.PublicURL(s3Key)
}
