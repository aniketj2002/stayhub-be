package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
)

type AdminRepository struct {
	db *db.DB
}

func NewAdminRepository(database *db.DB) interfaces.AdminRepository {
	return &AdminRepository{db: database}
}

const adminSelectCols = `
	id, email, password_hash, full_name, role, created_at, updated_at
`

func scanAdmin(row pgx.Row) (*models.AdminUser, error) {
	var a models.AdminUser
	err := row.Scan(
		&a.ID,
		&a.Email,
		&a.PasswordHash,
		&a.FullName,
		&a.Role,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *AdminRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error) {
	q := fmt.Sprintf(`SELECT %s FROM admin_users WHERE id = $1`, adminSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, id)
	return scanAdmin(row)
}

func (r *AdminRepository) FindByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	q := fmt.Sprintf(`SELECT %s FROM admin_users WHERE email = $1`, adminSelectCols)
	row := r.db.Conn(ctx).QueryRow(ctx, q, email)
	return scanAdmin(row)
}

func (r *AdminRepository) List(ctx context.Context) ([]models.AdminUser, error) {
	q := fmt.Sprintf(`SELECT %s FROM admin_users ORDER BY created_at ASC`, adminSelectCols)
	rows, err := r.db.Conn(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list admin_users: %w", err)
	}
	defer rows.Close()

	var out []models.AdminUser
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, fmt.Errorf("scan admin: %w", err)
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *AdminRepository) Create(ctx context.Context, admin models.AdminUser) (*models.AdminUser, error) {
	q := fmt.Sprintf(`
		INSERT INTO admin_users (id, email, password_hash, full_name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING %s
	`, adminSelectCols)
	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		admin.ID,
		admin.Email,
		admin.PasswordHash,
		admin.FullName,
		admin.Role,
		admin.CreatedAt,
		admin.UpdatedAt,
	)
	return scanAdmin(row)
}

func (r *AdminRepository) Update(ctx context.Context, admin models.AdminUser) (*models.AdminUser, error) {
	q := fmt.Sprintf(`
		UPDATE admin_users
		SET email = $2,
		    password_hash = $3,
		    full_name = $4,
		    role = $5,
		    updated_at = $6
		WHERE id = $1
		RETURNING %s
	`, adminSelectCols)
	row := r.db.Conn(ctx).QueryRow(
		ctx,
		q,
		admin.ID,
		admin.Email,
		admin.PasswordHash,
		admin.FullName,
		admin.Role,
		admin.UpdatedAt,
	)
	return scanAdmin(row)
}

func (r *AdminRepository) GetRolePermissions(ctx context.Context, role enums.AdminRole) ([]string, error) {
	q := `SELECT permission FROM admin_role_permissions WHERE role = $1 ORDER BY permission ASC`
	rows, err := r.db.Conn(ctx).Query(ctx, q, string(role))
	if err != nil {
		return nil, fmt.Errorf("query admin_role_permissions: %w", err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *AdminRepository) GetPermissionsByAdmin(ctx context.Context, adminID uuid.UUID) ([]models.AdminPermission, error) {
	q := `SELECT admin_id, permission, granted FROM admin_permissions WHERE admin_id = $1`
	rows, err := r.db.Conn(ctx).Query(ctx, q, adminID)
	if err != nil {
		return nil, fmt.Errorf("query admin_permissions: %w", err)
	}
	defer rows.Close()

	var out []models.AdminPermission
	for rows.Next() {
		var ap models.AdminPermission
		if err := rows.Scan(&ap.AdminID, &ap.Permission, &ap.Granted); err != nil {
			return nil, err
		}
		out = append(out, ap)
	}
	return out, rows.Err()
}

func (r *AdminRepository) SetPermission(ctx context.Context, adminID uuid.UUID, permission string, granted bool) error {
	q := `
		INSERT INTO admin_permissions (admin_id, permission, granted)
		VALUES ($1, $2, $3)
		ON CONFLICT (admin_id, permission) DO UPDATE
		SET granted = EXCLUDED.granted
	`
	_, err := r.db.Conn(ctx).Exec(ctx, q, adminID, permission, granted)
	return err
}

func (r *AdminRepository) HasPermission(ctx context.Context, adminID uuid.UUID, role enums.AdminRole, permission string) (bool, error) {
	// 1. Check direct override
	var granted bool
	err := r.db.Conn(ctx).QueryRow(
		ctx,
		`SELECT granted FROM admin_permissions WHERE admin_id = $1 AND permission = $2`,
		adminID,
		permission,
	).Scan(&granted)
	if err == nil {
		return granted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	// 2. Fall back to role permission
	if role == enums.AdminRoleSuperAdmin {
		return true, nil
	}

	var count int
	err = r.db.Conn(ctx).QueryRow(
		ctx,
		`SELECT count(*) FROM admin_role_permissions WHERE role = $1 AND permission = $2`,
		string(role),
		permission,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
