package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type ListingRepository struct {
	db *db.DB
}

func NewListingRepository(database *db.DB) interfaces.ListingRepository {
	return &ListingRepository{db: database}
}

func (r *ListingRepository) CreateListing(ctx context.Context, hostID uuid.UUID) (*models.Listing, error) {
	const q = `
		INSERT INTO listings (host_id, status)
		VALUES ($1, 'draft')
		RETURNING id, host_id, status, live_version_id, working_version_id, created_at, updated_at
	`
	var l models.Listing
	err := r.db.Conn(ctx).QueryRow(ctx, q, hostID).Scan(
		&l.ID,
		&l.HostID,
		&l.Status,
		&l.LiveVersionID,
		&l.WorkingVersionID,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create listing: %w", err)
	}
	return &l, nil
}

func (r *ListingRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Listing, error) {
	const q = `
		SELECT id, host_id, status, live_version_id, working_version_id, created_at, updated_at
		FROM listings
		WHERE id = $1
	`
	var l models.Listing
	err := r.db.Conn(ctx).QueryRow(ctx, q, id).Scan(
		&l.ID,
		&l.HostID,
		&l.Status,
		&l.LiveVersionID,
		&l.WorkingVersionID,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find listing by id: %w", err)
	}
	return &l, nil
}

func (r *ListingRepository) UpdatePointersAndStatus(
	ctx context.Context,
	id uuid.UUID,
	status enums.ListingStatus,
	liveVersionID, workingVersionID *uuid.UUID,
	now time.Time,
) (*models.Listing, error) {
	const q = `
		UPDATE listings
		SET status = $2,
		    live_version_id = $3,
		    working_version_id = $4,
		    updated_at = $5
		WHERE id = $1
		RETURNING id, host_id, status, live_version_id, working_version_id, created_at, updated_at
	`
	var l models.Listing
	err := r.db.Conn(ctx).QueryRow(ctx, q, id, status, liveVersionID, workingVersionID, now).Scan(
		&l.ID,
		&l.HostID,
		&l.Status,
		&l.LiveVersionID,
		&l.WorkingVersionID,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("update listing pointers and status: %w", err)
	}
	return &l, nil
}

const listingVersionSelectCols = `
	id, listing_id, version_no, status, wizard_step,
	property_type, title, description,
	address_line1, address_line2, city, region, TRIM(country), postal_code,
	timezone, max_guests, base_price_minor, cleaning_fee_minor, TRIM(currency),
	instant_book, to_char(check_in_time, 'HH24:MI'), to_char(check_out_time, 'HH24:MI'),
	house_rules, cancellation_policy, amenities, media_ids,
	submitted_at, decided_at, created_at, updated_at
`

func scanListingVersion(row pgx.Row) (*models.ListingVersion, error) {
	var v models.ListingVersion
	err := row.Scan(
		&v.ID,
		&v.ListingID,
		&v.VersionNo,
		&v.Status,
		&v.WizardStep,
		&v.PropertyType,
		&v.Title,
		&v.Description,
		&v.AddressLine1,
		&v.AddressLine2,
		&v.City,
		&v.Region,
		&v.Country,
		&v.PostalCode,
		&v.Timezone,
		&v.MaxGuests,
		&v.BasePriceMinor,
		&v.CleaningFeeMinor,
		&v.Currency,
		&v.InstantBook,
		&v.CheckInTime,
		&v.CheckOutTime,
		&v.HouseRules,
		&v.CancellationPolicy,
		&v.Amenities,
		&v.MediaIDs,
		&v.SubmittedAt,
		&v.DecidedAt,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	if v.Amenities == nil {
		v.Amenities = []string{}
	}
	if v.MediaIDs == nil {
		v.MediaIDs = []uuid.UUID{}
	}
	return &v, nil
}

func (r *ListingRepository) CreateVersion(ctx context.Context, v models.ListingVersion) (*models.ListingVersion, error) {
	if v.Amenities == nil {
		v.Amenities = []string{}
	}
	if v.MediaIDs == nil {
		v.MediaIDs = []uuid.UUID{}
	}
	if v.Timezone == "" {
		v.Timezone = "UTC"
	}
	if v.Currency == "" {
		v.Currency = "EUR"
	}
	if v.CheckInTime == "" {
		v.CheckInTime = "15:00"
	}
	if v.CheckOutTime == "" {
		v.CheckOutTime = "11:00"
	}
	if v.WizardStep <= 0 {
		v.WizardStep = 1
	}

	q := fmt.Sprintf(`
		INSERT INTO listing_versions (
			listing_id, version_no, status, wizard_step,
			property_type, title, description,
			address_line1, address_line2, city, region, country, postal_code,
			timezone, max_guests, base_price_minor, cleaning_fee_minor, currency,
			instant_book, check_in_time, check_out_time,
			house_rules, cancellation_policy, amenities, media_ids,
			submitted_at, decided_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20::time, $21::time,
			$22, $23, $24, $25,
			$26, $27
		)
		RETURNING %s
	`, listingVersionSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		v.ListingID,
		v.VersionNo,
		v.Status,
		v.WizardStep,
		v.PropertyType,
		v.Title,
		v.Description,
		v.AddressLine1,
		v.AddressLine2,
		v.City,
		v.Region,
		v.Country,
		v.PostalCode,
		v.Timezone,
		v.MaxGuests,
		v.BasePriceMinor,
		v.CleaningFeeMinor,
		v.Currency,
		v.InstantBook,
		v.CheckInTime,
		v.CheckOutTime,
		v.HouseRules,
		v.CancellationPolicy,
		v.Amenities,
		v.MediaIDs,
		v.SubmittedAt,
		v.DecidedAt,
	)

	created, err := scanListingVersion(row)
	if err != nil {
		return nil, fmt.Errorf("create listing_version: %w", err)
	}
	return created, nil
}

func (r *ListingRepository) FindVersionByID(ctx context.Context, id uuid.UUID) (*models.ListingVersion, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM listing_versions
		WHERE id = $1
	`, listingVersionSelectCols)

	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	v, err := scanListingVersion(row)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *ListingRepository) UpdateVersion(ctx context.Context, v models.ListingVersion) (*models.ListingVersion, error) {
	if v.Amenities == nil {
		v.Amenities = []string{}
	}
	if v.MediaIDs == nil {
		v.MediaIDs = []uuid.UUID{}
	}

	q := fmt.Sprintf(`
		UPDATE listing_versions
		SET status = $2,
		    wizard_step = $3,
		    property_type = $4,
		    title = $5,
		    description = $6,
		    address_line1 = $7,
		    address_line2 = $8,
		    city = $9,
		    region = $10,
		    country = $11,
		    postal_code = $12,
		    timezone = $13,
		    max_guests = $14,
		    base_price_minor = $15,
		    cleaning_fee_minor = $16,
		    currency = $17,
		    instant_book = $18,
		    check_in_time = $19::time,
		    check_out_time = $20::time,
		    house_rules = $21,
		    cancellation_policy = $22,
		    amenities = $23,
		    media_ids = $24,
		    submitted_at = $25,
		    decided_at = $26,
		    updated_at = $27
		WHERE id = $1
		RETURNING %s
	`, listingVersionSelectCols)

	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		v.ID,
		v.Status,
		v.WizardStep,
		v.PropertyType,
		v.Title,
		v.Description,
		v.AddressLine1,
		v.AddressLine2,
		v.City,
		v.Region,
		v.Country,
		v.PostalCode,
		v.Timezone,
		v.MaxGuests,
		v.BasePriceMinor,
		v.CleaningFeeMinor,
		v.Currency,
		v.InstantBook,
		v.CheckInTime,
		v.CheckOutTime,
		v.HouseRules,
		v.CancellationPolicy,
		v.Amenities,
		v.MediaIDs,
		v.SubmittedAt,
		v.DecidedAt,
		v.UpdatedAt,
	)

	updated, err := scanListingVersion(row)
	if err != nil {
		return nil, fmt.Errorf("update listing_version: %w", err)
	}
	return updated, nil
}

func (r *ListingRepository) ReplaceCheckResults(
	ctx context.Context,
	versionID uuid.UUID,
	results []models.ListingCheckResult,
) ([]models.ListingCheckResult, error) {
	const deleteQ = `DELETE FROM listing_check_results WHERE listing_version_id = $1`
	if _, err := r.db.Conn(ctx).Exec(ctx, deleteQ, versionID); err != nil {
		return nil, fmt.Errorf("delete old check results: %w", err)
	}

	const insertQ = `
		INSERT INTO listing_check_results (listing_version_id, check_code, outcome, detail)
		VALUES ($1, $2, $3, $4)
		RETURNING id, listing_version_id, check_code, outcome, detail, created_at
	`
	saved := make([]models.ListingCheckResult, 0, len(results))
	for _, item := range results {
		var rec models.ListingCheckResult
		if err := r.db.Conn(ctx).QueryRow(
			ctx,
			insertQ,
			versionID,
			item.CheckCode,
			item.Outcome,
			item.Detail,
		).Scan(
			&rec.ID,
			&rec.ListingVersionID,
			&rec.CheckCode,
			&rec.Outcome,
			&rec.Detail,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("insert check result %s: %w", item.CheckCode, err)
		}
		saved = append(saved, rec)
	}
	return saved, nil
}

func (r *ListingRepository) ListCheckResultsByVersion(ctx context.Context, versionID uuid.UUID) ([]models.ListingCheckResult, error) {
	const q = `
		SELECT id, listing_version_id, check_code, outcome, detail, created_at
		FROM listing_check_results
		WHERE listing_version_id = $1
		ORDER BY created_at ASC, check_code ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q, versionID)
	if err != nil {
		return nil, fmt.Errorf("list check results: %w", err)
	}
	defer rows.Close()

	out := make([]models.ListingCheckResult, 0)
	for rows.Next() {
		var rec models.ListingCheckResult
		if err := rows.Scan(
			&rec.ID,
			&rec.ListingVersionID,
			&rec.CheckCode,
			&rec.Outcome,
			&rec.Detail,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan check result: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *ListingRepository) SearchLiveListings(ctx context.Context, filter dtos.SearchListingsFilter) ([]dtos.SearchListingRow, error) {
	clauses := []string{
		"l.status = 'live'",
		"l.live_version_id IS NOT NULL",
	}
	var args []any
	argIdx := 1

	if city := strings.TrimSpace(filter.City); city != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(lv.city) LIKE LOWER($%d)", argIdx))
		args = append(args, "%"+city+"%")
		argIdx++
	}
	if country := strings.TrimSpace(filter.Country); country != "" {
		clauses = append(clauses, fmt.Sprintf("UPPER(TRIM(lv.country)) = UPPER($%d)", argIdx))
		args = append(args, country)
		argIdx++
	}
	if filter.Guests > 0 {
		clauses = append(clauses, fmt.Sprintf("lv.max_guests >= $%d", argIdx))
		args = append(args, filter.Guests)
		argIdx++
	}
	if propType := strings.TrimSpace(filter.PropertyType); propType != "" {
		clauses = append(clauses, fmt.Sprintf("LOWER(lv.property_type) = LOWER($%d)", argIdx))
		args = append(args, propType)
		argIdx++
	}
	if filter.InstantBook != nil {
		clauses = append(clauses, fmt.Sprintf("lv.instant_book = $%d", argIdx))
		args = append(args, *filter.InstantBook)
		argIdx++
	}
	if filter.MinPriceMinor != nil {
		clauses = append(clauses, fmt.Sprintf("lv.base_price_minor >= $%d", argIdx))
		args = append(args, *filter.MinPriceMinor)
		argIdx++
	}
	if filter.MaxPriceMinor != nil {
		clauses = append(clauses, fmt.Sprintf("lv.base_price_minor <= $%d", argIdx))
		args = append(args, *filter.MaxPriceMinor)
		argIdx++
	}
	if len(filter.Amenities) > 0 {
		clauses = append(clauses, fmt.Sprintf("lv.amenities @> $%d", argIdx))
		args = append(args, filter.Amenities)
		argIdx++
	}
	if filter.CheckIn != nil && filter.CheckOut != nil {
		clauses = append(clauses, fmt.Sprintf(`
			NOT EXISTS (
				SELECT 1
				FROM listing_calendar lc
				WHERE lc.listing_id = l.id
				  AND lc.night >= $%d
				  AND lc.night < $%d
			)
		`, argIdx, argIdx+1))
		args = append(args, filter.CheckIn.Format("2006-01-02"), filter.CheckOut.Format("2006-01-02"))
		argIdx += 2
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	q := fmt.Sprintf(`
		SELECT
			l.id, l.host_id, l.status, l.live_version_id, l.working_version_id, l.created_at, l.updated_at,
			lv.id, lv.listing_id, lv.version_no, lv.status, lv.wizard_step,
			lv.property_type, lv.title, lv.description,
			lv.address_line1, lv.address_line2, lv.city, lv.region, TRIM(lv.country), lv.postal_code,
			lv.timezone, lv.max_guests, lv.base_price_minor, lv.cleaning_fee_minor, TRIM(lv.currency),
			lv.instant_book, to_char(lv.check_in_time, 'HH24:MI'), to_char(lv.check_out_time, 'HH24:MI'),
			lv.house_rules, lv.cancellation_policy, lv.amenities, lv.media_ids,
			lv.submitted_at, lv.decided_at, lv.created_at, lv.updated_at
		FROM listings l
		JOIN listing_versions lv ON lv.id = l.live_version_id
		WHERE %s
		ORDER BY l.created_at DESC, l.id ASC
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Conn(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search live listings: %w", err)
	}
	defer rows.Close()

	out := make([]dtos.SearchListingRow, 0)
	for rows.Next() {
		var row dtos.SearchListingRow
		if err := rows.Scan(
			&row.Listing.ID,
			&row.Listing.HostID,
			&row.Listing.Status,
			&row.Listing.LiveVersionID,
			&row.Listing.WorkingVersionID,
			&row.Listing.CreatedAt,
			&row.Listing.UpdatedAt,
			&row.LiveVersion.ID,
			&row.LiveVersion.ListingID,
			&row.LiveVersion.VersionNo,
			&row.LiveVersion.Status,
			&row.LiveVersion.WizardStep,
			&row.LiveVersion.PropertyType,
			&row.LiveVersion.Title,
			&row.LiveVersion.Description,
			&row.LiveVersion.AddressLine1,
			&row.LiveVersion.AddressLine2,
			&row.LiveVersion.City,
			&row.LiveVersion.Region,
			&row.LiveVersion.Country,
			&row.LiveVersion.PostalCode,
			&row.LiveVersion.Timezone,
			&row.LiveVersion.MaxGuests,
			&row.LiveVersion.BasePriceMinor,
			&row.LiveVersion.CleaningFeeMinor,
			&row.LiveVersion.Currency,
			&row.LiveVersion.InstantBook,
			&row.LiveVersion.CheckInTime,
			&row.LiveVersion.CheckOutTime,
			&row.LiveVersion.HouseRules,
			&row.LiveVersion.CancellationPolicy,
			&row.LiveVersion.Amenities,
			&row.LiveVersion.MediaIDs,
			&row.LiveVersion.SubmittedAt,
			&row.LiveVersion.DecidedAt,
			&row.LiveVersion.CreatedAt,
			&row.LiveVersion.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan search listing row: %w", err)
		}
		if row.LiveVersion.Amenities == nil {
			row.LiveVersion.Amenities = []string{}
		}
		if row.LiveVersion.MediaIDs == nil {
			row.LiveVersion.MediaIDs = []uuid.UUID{}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
