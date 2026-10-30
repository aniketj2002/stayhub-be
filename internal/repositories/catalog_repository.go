package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type CatalogRepository struct {
	db *db.DB
}

func NewCatalogRepository(database *db.DB) interfaces.CatalogRepository {
	return &CatalogRepository{db: database}
}

func (r *CatalogRepository) ListAmenities(ctx context.Context) ([]models.Amenity, error) {
	const q = `
		SELECT code, label, category
		FROM amenities
		ORDER BY category ASC, code ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list amenities: %w", err)
	}
	defer rows.Close()

	out := make([]models.Amenity, 0)
	for rows.Next() {
		var a models.Amenity
		if err := rows.Scan(&a.Code, &a.Label, &a.Category); err != nil {
			return nil, fmt.Errorf("scan amenity: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *CatalogRepository) ListCancellationPolicies(ctx context.Context) ([]models.CancellationPolicy, error) {
	const policiesQ = `
		SELECT code, name, description
		FROM cancellation_policies
		ORDER BY CASE code
			WHEN 'flexible' THEN 1
			WHEN 'moderate' THEN 2
			WHEN 'firm'     THEN 3
			WHEN 'strict'   THEN 4
			ELSE 5
		END, code ASC
	`
	rows, err := r.db.Conn(ctx).Query(ctx, policiesQ)
	if err != nil {
		return nil, fmt.Errorf("list cancellation policies: %w", err)
	}
	defer rows.Close()

	policies := make([]models.CancellationPolicy, 0)
	indexByCode := make(map[string]int)
	for rows.Next() {
		var p models.CancellationPolicy
		if err := rows.Scan(&p.Code, &p.Name, &p.Description); err != nil {
			return nil, fmt.Errorf("scan cancellation policy: %w", err)
		}
		p.Rules = make([]models.CancellationPolicyRule, 0)
		indexByCode[p.Code] = len(policies)
		policies = append(policies, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	const rulesQ = `
		SELECT policy_code, min_hours_before, refund_pct, refunds_service_fee
		FROM cancellation_policy_rules
		ORDER BY policy_code ASC, min_hours_before DESC
	`
	ruleRows, err := r.db.Conn(ctx).Query(ctx, rulesQ)
	if err != nil {
		return nil, fmt.Errorf("list cancellation policy rules: %w", err)
	}
	defer ruleRows.Close()

	for ruleRows.Next() {
		var rule models.CancellationPolicyRule
		if err := ruleRows.Scan(
			&rule.PolicyCode,
			&rule.MinHoursBefore,
			&rule.RefundPct,
			&rule.RefundsServiceFee,
		); err != nil {
			return nil, fmt.Errorf("scan cancellation policy rule: %w", err)
		}
		if idx, ok := indexByCode[rule.PolicyCode]; ok {
			policies[idx].Rules = append(policies[idx].Rules, rule)
		}
	}
	return policies, ruleRows.Err()
}

func (r *CatalogRepository) FindCancellationPolicyByCode(ctx context.Context, code string) (*models.CancellationPolicy, error) {
	const policyQ = `
		SELECT code, name, description
		FROM cancellation_policies
		WHERE code = $1
	`
	var p models.CancellationPolicy
	err := r.db.Conn(ctx).QueryRow(ctx, policyQ, code).Scan(&p.Code, &p.Name, &p.Description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("find cancellation policy by code: %w", err)
	}

	const rulesQ = `
		SELECT policy_code, min_hours_before, refund_pct, refunds_service_fee
		FROM cancellation_policy_rules
		WHERE policy_code = $1
		ORDER BY min_hours_before DESC
	`
	ruleRows, err := r.db.Conn(ctx).Query(ctx, rulesQ, code)
	if err != nil {
		return nil, fmt.Errorf("query policy rules: %w", err)
	}
	defer ruleRows.Close()

	p.Rules = make([]models.CancellationPolicyRule, 0)
	for ruleRows.Next() {
		var rule models.CancellationPolicyRule
		if err := ruleRows.Scan(
			&rule.PolicyCode,
			&rule.MinHoursBefore,
			&rule.RefundPct,
			&rule.RefundsServiceFee,
		); err != nil {
			return nil, fmt.Errorf("scan policy rule: %w", err)
		}
		p.Rules = append(p.Rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, err
	}
	return &p, nil
}
