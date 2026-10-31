package models

import (
	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/enums"
)

type AdminRolePermission struct {
	Role       enums.AdminRole `db:"role" json:"role"`
	Permission string          `db:"permission" json:"permission"`
}

type AdminPermission struct {
	AdminID    uuid.UUID `db:"admin_id" json:"admin_id"`
	Permission string    `db:"permission" json:"permission"`
	Granted    bool      `db:"granted" json:"granted"`
}
