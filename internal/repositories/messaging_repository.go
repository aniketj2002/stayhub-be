package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type MessagingRepository struct {
	db *db.DB
}

func NewMessagingRepository(database *db.DB) interfaces.MessagingRepository {
	return &MessagingRepository{db: database}
}

func (r *MessagingRepository) GetOrCreateConversation(
	ctx context.Context,
	reservationID, listingID, guestID uuid.UUID,
) (*models.Conversation, error) {
	query := `
		INSERT INTO conversations (reservation_id, listing_id, guest_id, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (reservation_id) DO UPDATE SET updated_at = now()
		RETURNING id, reservation_id, listing_id, guest_id, created_at, updated_at;
	`
	var c models.Conversation
	err := r.db.Pool.QueryRow(ctx, query, reservationID, listingID, guestID).Scan(
		&c.ID,
		&c.ReservationID,
		&c.ListingID,
		&c.GuestID,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) FindConversationByID(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	query := `
		SELECT id, reservation_id, listing_id, guest_id, created_at, updated_at
		FROM conversations
		WHERE id = $1;
	`
	var c models.Conversation
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.ReservationID,
		&c.ListingID,
		&c.GuestID,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) FindConversationByReservation(ctx context.Context, reservationID uuid.UUID) (*models.Conversation, error) {
	query := `
		SELECT id, reservation_id, listing_id, guest_id, created_at, updated_at
		FROM conversations
		WHERE reservation_id = $1;
	`
	var c models.Conversation
	err := r.db.Pool.QueryRow(ctx, query, reservationID).Scan(
		&c.ID,
		&c.ReservationID,
		&c.ListingID,
		&c.GuestID,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) ListConversationsByUser(ctx context.Context, userID uuid.UUID) ([]models.Conversation, error) {
	query := `
		SELECT c.id, c.reservation_id, c.listing_id, c.guest_id, c.created_at, c.updated_at
		FROM conversations c
		WHERE c.guest_id = $1
		   OR EXISTS (
		       SELECT 1 FROM listings l
		       WHERE l.id = c.listing_id
		         AND l.host_id = $1
		   )
		   OR EXISTS (
		       SELECT 1 FROM listing_hosts lh
		       WHERE lh.listing_id = c.listing_id
		         AND lh.user_id = $1
		         AND (lh.role = 'owner' OR lh.can_message_guests = true)
		   )
		ORDER BY c.updated_at DESC;
	`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Conversation
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(
			&c.ID,
			&c.ReservationID,
			&c.ListingID,
			&c.GuestID,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *MessagingRepository) InsertMessage(ctx context.Context, msg models.Message) (*models.Message, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// A nil slice is encoded as SQL NULL. Both columns are NOT NULL and default to '{}'.
	if msg.MediaIDs == nil {
		msg.MediaIDs = []uuid.UUID{}
	}
	if msg.Flags == nil {
		msg.Flags = []string{}
	}

	msgQuery := `
		INSERT INTO messages (conversation_id, sender_id, body, media_ids, flags, created_at)
		VALUES ($1, $2, $3, COALESCE($4, '{}'::uuid[]), COALESCE($5, '{}'::text[]), $6)
		RETURNING id, conversation_id, sender_id, body, media_ids, flags, created_at;
	`
	var m models.Message
	err = tx.QueryRow(
		ctx,
		msgQuery,
		msg.ConversationID,
		msg.SenderID,
		msg.Body,
		msg.MediaIDs,
		msg.Flags,
		msg.CreatedAt,
	).Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.Body,
		&m.MediaIDs,
		&m.Flags,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if m.MediaIDs == nil {
		m.MediaIDs = []uuid.UUID{}
	}
	if m.Flags == nil {
		m.Flags = []string{}
	}

	convQuery := `UPDATE conversations SET updated_at = $1 WHERE id = $2;`
	if _, err := tx.Exec(ctx, convQuery, msg.CreatedAt, msg.ConversationID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &m, nil
}

func (r *MessagingRepository) ListMessagesByConversation(
	ctx context.Context,
	conversationID uuid.UUID,
	before *time.Time,
	limit int,
) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, conversation_id, sender_id, body, media_ids, flags, created_at
		FROM messages
		WHERE conversation_id = $1
		  AND ($2::timestamptz IS NULL OR created_at < $2)
		ORDER BY created_at DESC
		LIMIT $3;
	`
	rows, err := r.db.Pool.Query(ctx, query, conversationID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID,
			&m.ConversationID,
			&m.SenderID,
			&m.Body,
			&m.MediaIDs,
			&m.Flags,
			&m.CreatedAt,
		); err != nil {
			return nil, err
		}
		if m.MediaIDs == nil {
			m.MediaIDs = []uuid.UUID{}
		}
		if m.Flags == nil {
			m.Flags = []string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MessagingRepository) GetLatestMessage(ctx context.Context, conversationID uuid.UUID) (*models.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, body, media_ids, flags, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT 1;
	`
	var m models.Message
	err := r.db.Pool.QueryRow(ctx, query, conversationID).Scan(
		&m.ID,
		&m.ConversationID,
		&m.SenderID,
		&m.Body,
		&m.MediaIDs,
		&m.Flags,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if m.MediaIDs == nil {
		m.MediaIDs = []uuid.UUID{}
	}
	if m.Flags == nil {
		m.Flags = []string{}
	}
	return &m, nil
}

func (r *MessagingRepository) GetUnreadCount(ctx context.Context, conversationID, userID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM messages m
		LEFT JOIN conversation_reads cr
		       ON cr.conversation_id = m.conversation_id
		      AND cr.user_id = $2
		WHERE m.conversation_id = $1
		  AND m.sender_id != $2
		  AND (cr.last_read_at IS NULL OR m.created_at > cr.last_read_at);
	`
	var count int
	err := r.db.Pool.QueryRow(ctx, query, conversationID, userID).Scan(&count)
	return count, err
}

func (r *MessagingRepository) MarkConversationRead(ctx context.Context, conversationID, userID uuid.UUID, readAt time.Time) error {
	query := `
		INSERT INTO conversation_reads (conversation_id, user_id, last_read_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (conversation_id, user_id)
		DO UPDATE SET last_read_at = EXCLUDED.last_read_at;
	`
	_, err := r.db.Pool.Exec(ctx, query, conversationID, userID, readAt)
	return err
}
