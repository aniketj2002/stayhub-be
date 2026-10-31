package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type AuditRepository struct {
	db *db.DB
}

func NewAuditRepository(database *db.DB) interfaces.AuditRepository {
	return &AuditRepository{db: database}
}

func (r *AuditRepository) Insert(ctx context.Context, p dtos.AuditInsertParams) error {
	const q = `
		INSERT INTO event_log (
			occurred_at, actor_type, actor_id, action, object_type, object_id,
			before_state, after_state, reason, request_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Conn(ctx).Exec(
		ctx,
		q,
		p.OccurredAt,
		string(p.ActorType),
		p.ActorID,
		p.Action,
		p.ObjectType,
		p.ObjectID,
		nullableJSON(p.BeforeBytes),
		nullableJSON(p.AfterBytes),
		p.Reason,
		p.RequestID,
	)
	if err != nil {
		return fmt.Errorf("insert event_log: %w", err)
	}
	return nil
}

func (r *AuditRepository) FindMany(ctx context.Context, f dtos.EventListFilter) ([]models.EventLog, error) {
	var clauses []string
	var args []any
	argIdx := 1

	if f.ObjectType != "" {
		clauses = append(clauses, fmt.Sprintf("object_type = $%d", argIdx))
		args = append(args, f.ObjectType)
		argIdx++
	}
	if f.ObjectID != nil {
		clauses = append(clauses, fmt.Sprintf("object_id = $%d", argIdx))
		args = append(args, *f.ObjectID)
		argIdx++
	}
	if f.ActorType != "" {
		if strings.EqualFold(f.ActorType, "user") {
			clauses = append(clauses, "actor_type IN ('guest', 'host', 'cohost')")
		} else if strings.EqualFold(f.ActorType, "worker") {
			clauses = append(clauses, "actor_type = 'system'")
		} else {
			clauses = append(clauses, fmt.Sprintf("actor_type::text = $%d", argIdx))
			args = append(args, strings.ToLower(f.ActorType))
			argIdx++
		}
	}
	if f.ActorID != nil {
		clauses = append(clauses, fmt.Sprintf("actor_id = $%d", argIdx))
		args = append(args, *f.ActorID)
		argIdx++
	}
	if f.Action != "" {
		clauses = append(clauses, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, f.Action)
		argIdx++
	}
	if f.Query != "" {
		clauses = append(clauses, fmt.Sprintf(`(
			action ILIKE $%d ESCAPE '\'
			OR object_type ILIKE $%d ESCAPE '\'
			OR actor_type::text ILIKE $%d ESCAPE '\'
			OR COALESCE(reason, '') ILIKE $%d ESCAPE '\'
			OR object_id::text ILIKE $%d ESCAPE '\'
			OR COALESCE(actor_id::text, '') ILIKE $%d ESCAPE '\'
		)`, argIdx, argIdx, argIdx, argIdx, argIdx, argIdx))
		args = append(args, likePattern(f.Query))
		argIdx++
	}
	if f.From != nil {
		clauses = append(clauses, fmt.Sprintf("occurred_at >= $%d", argIdx))
		args = append(args, *f.From)
		argIdx++
	}
	if f.To != nil {
		clauses = append(clauses, fmt.Sprintf("occurred_at <= $%d", argIdx))
		args = append(args, *f.To)
		argIdx++
	}

	whereSQL := ""
	if len(clauses) > 0 {
		whereSQL = "WHERE " + strings.Join(clauses, " AND ")
	}

	q := fmt.Sprintf(`
		SELECT id, occurred_at, actor_type, actor_id, action, object_type, object_id,
		       before_state, after_state, reason, request_id
		FROM event_log
		%s
		ORDER BY occurred_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.Conn(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query event_log: %w", err)
	}
	defer rows.Close()

	out := make([]models.EventLog, 0)
	for rows.Next() {
		var rec models.EventLog
		var beforeRaw, afterRaw []byte
		if err := rows.Scan(
			&rec.ID,
			&rec.OccurredAt,
			&rec.ActorType,
			&rec.ActorID,
			&rec.Action,
			&rec.ObjectType,
			&rec.ObjectID,
			&beforeRaw,
			&afterRaw,
			&rec.Reason,
			&rec.RequestID,
		); err != nil {
			return nil, fmt.Errorf("scan event_log: %w", err)
		}
		if len(beforeRaw) > 0 {
			rec.BeforeState = json.RawMessage(beforeRaw)
		}
		if len(afterRaw) > 0 {
			rec.AfterState = json.RawMessage(afterRaw)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func likePattern(q string) string {
	var b strings.Builder
	b.WriteByte('%')
	for _, r := range q {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('%')
	return b.String()
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
