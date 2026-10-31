package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/dtos"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type PaymentRepository struct {
	db *db.DB
}

func NewPaymentRepository(database *db.DB) interfaces.PaymentRepository {
	return &PaymentRepository{db: database}
}

const paymentSelectCols = `
	id, reservation_id, kind, status, amount_minor, TRIM(currency),
	idempotency_key, simulated_ref, related_payment_id,
	refund_pct, refund_basis, failure_reason, created_at, updated_at
`

func scanPayment(row pgx.Row) (*models.Payment, error) {
	var p models.Payment
	var basisRaw []byte
	err := row.Scan(
		&p.ID,
		&p.ReservationID,
		&p.Kind,
		&p.Status,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.SimulatedRef,
		&p.RelatedPaymentID,
		&p.RefundPct,
		&basisRaw,
		&p.FailureReason,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	if len(basisRaw) > 0 {
		p.RefundBasis = json.RawMessage(basisRaw)
	}
	return &p, nil
}

func (r *PaymentRepository) CreatePayment(ctx context.Context, p models.Payment) (*models.Payment, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	q := fmt.Sprintf(`
		INSERT INTO payments (
			id, reservation_id, kind, status, amount_minor, currency,
			idempotency_key, simulated_ref, related_payment_id,
			refund_pct, refund_basis, failure_reason
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12
		)
		RETURNING %s
	`, paymentSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		p.ID,
		p.ReservationID,
		p.Kind,
		p.Status,
		p.AmountMinor,
		p.Currency,
		p.IdempotencyKey,
		p.SimulatedRef,
		p.RelatedPaymentID,
		p.RefundPct,
		nullableJSON(p.RefundBasis),
		p.FailureReason,
	)
	created, err := scanPayment(row)
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return created, nil
}

func (r *PaymentRepository) FindChargeByReservation(ctx context.Context, reservationID uuid.UUID) (*models.Payment, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM payments
		WHERE reservation_id = $1 AND kind = 'charge' AND status = 'succeeded'
		ORDER BY created_at DESC
		LIMIT 1
	`, paymentSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, reservationID)
	return scanPayment(row)
}

func (r *PaymentRepository) ListPaymentsByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.Payment, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM payments
		WHERE reservation_id = $1
		ORDER BY created_at ASC
	`, paymentSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, reservationID)
	if err != nil {
		return nil, fmt.Errorf("list payments by reservation: %w", err)
	}
	defer rows.Close()

	out := make([]models.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

const payoutSelectCols = `
	id, reservation_id, host_id, status,
	gross_minor, host_fee_minor, net_minor, TRIM(currency),
	release_at, released_at, held_reason, held_by,
	idempotency_key, simulated_ref, created_at, updated_at
`

func scanPayout(row pgx.Row) (*models.Payout, error) {
	var p models.Payout
	err := row.Scan(
		&p.ID,
		&p.ReservationID,
		&p.HostID,
		&p.Status,
		&p.GrossMinor,
		&p.HostFeeMinor,
		&p.NetMinor,
		&p.Currency,
		&p.ReleaseAt,
		&p.ReleasedAt,
		&p.HeldReason,
		&p.HeldBy,
		&p.IdempotencyKey,
		&p.SimulatedRef,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepository) FindPayoutByID(ctx context.Context, id uuid.UUID) (*models.Payout, error) {
	q := fmt.Sprintf(`SELECT %s FROM payouts WHERE id = $1`, payoutSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanPayout(row)
}

func (r *PaymentRepository) CreatePayout(ctx context.Context, p models.Payout) (*models.Payout, error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	q := fmt.Sprintf(`
		INSERT INTO payouts (
			id, reservation_id, host_id, status,
			gross_minor, host_fee_minor, net_minor, currency,
			release_at, released_at, held_reason, held_by,
			idempotency_key, simulated_ref
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, $14
		)
		RETURNING %s
	`, payoutSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		p.ID,
		p.ReservationID,
		p.HostID,
		p.Status,
		p.GrossMinor,
		p.HostFeeMinor,
		p.NetMinor,
		p.Currency,
		p.ReleaseAt,
		p.ReleasedAt,
		p.HeldReason,
		p.HeldBy,
		p.IdempotencyKey,
		p.SimulatedRef,
	)
	created, err := scanPayout(row)
	if err != nil {
		return nil, fmt.Errorf("create payout: %w", err)
	}
	return created, nil
}

func (r *PaymentRepository) FindPayoutByReservation(ctx context.Context, reservationID uuid.UUID) (*models.Payout, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM payouts
		WHERE reservation_id = $1
	`, payoutSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, reservationID)
	return scanPayout(row)
}

func (r *PaymentRepository) FindPayoutByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Payout, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM payouts
		WHERE id = $1
		FOR UPDATE
	`, payoutSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanPayout(row)
}

func (r *PaymentRepository) UpdatePayout(ctx context.Context, p models.Payout) (*models.Payout, error) {
	q := fmt.Sprintf(`
		UPDATE payouts
		SET status = $2,
		    gross_minor = $3,
		    host_fee_minor = $4,
		    net_minor = $5,
		    release_at = $6,
		    released_at = $7,
		    held_reason = $8,
		    held_by = $9,
		    simulated_ref = $10,
		    updated_at = $11
		WHERE id = $1
		RETURNING %s
	`, payoutSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		p.ID,
		p.Status,
		p.GrossMinor,
		p.HostFeeMinor,
		p.NetMinor,
		p.ReleaseAt,
		p.ReleasedAt,
		p.HeldReason,
		p.HeldBy,
		p.SimulatedRef,
		p.UpdatedAt,
	)
	updated, err := scanPayout(row)
	if err != nil {
		return nil, fmt.Errorf("update payout: %w", err)
	}
	return updated, nil
}

func (r *PaymentRepository) ListPayoutsByListingIDs(ctx context.Context, listingIDs []uuid.UUID) ([]dtos.EarningsPayoutItem, error) {
	if len(listingIDs) == 0 {
		return []dtos.EarningsPayoutItem{}, nil
	}
	const q = `
		SELECT
			p.id, p.reservation_id, p.host_id, p.status,
			p.gross_minor, p.host_fee_minor, p.net_minor, TRIM(p.currency),
			p.release_at, p.released_at, p.held_reason,
			p.idempotency_key, p.simulated_ref, p.created_at, p.updated_at,
			res.code, res.listing_id, COALESCE(lv.title, 'Untitled Listing'),
			to_char(res.check_in, 'YYYY-MM-DD'), to_char(res.check_out, 'YYYY-MM-DD'),
			COALESCE(u.full_name, 'Guest')
		FROM payouts p
		JOIN reservations res ON res.id = p.reservation_id
		LEFT JOIN listing_versions lv ON lv.id = res.listing_version_id
		LEFT JOIN users u ON u.id = res.guest_id
		WHERE res.listing_id = ANY($1)
		ORDER BY p.created_at DESC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, listingIDs)
	if err != nil {
		return nil, fmt.Errorf("list payouts by listing_ids: %w", err)
	}
	defer rows.Close()

	out := make([]dtos.EarningsPayoutItem, 0)
	for rows.Next() {
		var item dtos.EarningsPayoutItem
		if err := rows.Scan(
			&item.Payout.ID,
			&item.Payout.ReservationID,
			&item.Payout.HostID,
			&item.Payout.Status,
			&item.Payout.GrossMinor,
			&item.Payout.HostFeeMinor,
			&item.Payout.NetMinor,
			&item.Payout.Currency,
			&item.Payout.ReleaseAt,
			&item.Payout.ReleasedAt,
			&item.Payout.HeldReason,
			&item.Payout.IdempotencyKey,
			&item.Payout.SimulatedRef,
			&item.Payout.CreatedAt,
			&item.Payout.UpdatedAt,
			&item.ReservationCode,
			&item.ListingID,
			&item.ListingTitle,
			&item.CheckIn,
			&item.CheckOut,
			&item.GuestName,
		); err != nil {
			return nil, fmt.Errorf("scan earnings payout item: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PaymentRepository) ListDuePayouts(ctx context.Context, now time.Time, limit int) ([]models.Payout, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM payouts
		WHERE status = 'scheduled'
		  AND release_at IS NOT NULL
		  AND release_at <= $1
		ORDER BY release_at ASC
		LIMIT $2
	`, payoutSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due payouts: %w", err)
	}
	defer rows.Close()

	out := make([]models.Payout, 0)
	for rows.Next() {
		p, err := scanPayout(rows)
		if err != nil {
			return nil, fmt.Errorf("scan due payout: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// InsertLedgerEntries validates that every txn_id balances debit == credit before persisting.
func (r *PaymentRepository) InsertLedgerEntries(ctx context.Context, entries []models.LedgerEntry) error {
	if len(entries) == 0 {
		return nil
	}

	balances := make(map[uuid.UUID]int64)
	for _, e := range entries {
		if e.AmountMinor <= 0 {
			return fmt.Errorf("ledger entry amount_minor must be > 0 (got %d)", e.AmountMinor)
		}
		switch e.Direction {
		case enums.LedgerDirectionDebit:
			balances[e.TxnID] += e.AmountMinor
		case enums.LedgerDirectionCredit:
			balances[e.TxnID] -= e.AmountMinor
		default:
			return fmt.Errorf("invalid ledger direction %q", e.Direction)
		}
	}
	for _, diff := range balances {
		if diff != 0 {
			return apperrors.ErrLedgerUnbalanced
		}
	}

	const insertQ = `
		INSERT INTO ledger_entries (
			txn_id, reservation_id, payment_id, payout_id,
			account, direction, amount_minor, currency
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for _, e := range entries {
		if _, err := r.db.Conn(ctx).Exec(
			ctx,
			insertQ,
			e.TxnID,
			e.ReservationID,
			e.PaymentID,
			e.PayoutID,
			e.Account,
			string(e.Direction),
			e.AmountMinor,
			e.Currency,
		); err != nil {
			return fmt.Errorf("insert ledger entry: %w", err)
		}
	}
	return nil
}

func (r *PaymentRepository) ListLedgerByReservation(ctx context.Context, reservationID uuid.UUID) ([]models.LedgerEntry, error) {
	const q = `
		SELECT id, txn_id, reservation_id, payment_id, payout_id,
		       account, direction, amount_minor, TRIM(currency), created_at
		FROM ledger_entries
		WHERE reservation_id = $1
		ORDER BY id ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, reservationID)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	defer rows.Close()

	out := make([]models.LedgerEntry, 0)
	for rows.Next() {
		var le models.LedgerEntry
		if err := rows.Scan(
			&le.ID,
			&le.TxnID,
			&le.ReservationID,
			&le.PaymentID,
			&le.PayoutID,
			&le.Account,
			&le.Direction,
			&le.AmountMinor,
			&le.Currency,
			&le.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		out = append(out, le)
	}
	return out, rows.Err()
}
