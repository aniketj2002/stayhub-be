package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/models"
)

type MediaRepository interface {
	CreatePending(ctx context.Context, p dtos.CreateMediaParams) (*models.Media, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.Media, error)
	FindByIDs(ctx context.Context, ids []uuid.UUID) ([]models.Media, error)
	ConfirmUpload(ctx context.Context, id uuid.UUID, width, height *int) (*models.Media, error)
}

type MediaService interface {
	PresignUpload(ctx context.Context, actor models.AuthActor, in dtos.PresignMediaInput) (*dtos.PresignMediaResponse, error)
	UploadContent(ctx context.Context, mediaID uuid.UUID, contentType string, data []byte) error
	ConfirmUpload(ctx context.Context, actor models.AuthActor, mediaID uuid.UUID, in dtos.ConfirmMediaInput) (*dtos.MediaResponse, error)
	GetByID(ctx context.Context, mediaID uuid.UUID) (*dtos.MediaResponse, error)
	ResolveLocalFilePath(s3Key string) (string, error)
	PublicURL(s3Key string) string
}
