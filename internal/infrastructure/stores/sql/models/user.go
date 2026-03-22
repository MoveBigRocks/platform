package models

import (
	"time"
)

// User is the sqlx model for users
type User struct {
	ID     string  `db:"id"`
	Email  string  `db:"email"`
	Name   *string `db:"name"`
	Avatar *string `db:"avatar"`

	// Instance-level role (optional - most users won't have this)
	InstanceRole *string `db:"instance_role"`

	// Authentication & Security
	IsActive      bool       `db:"is_active"`
	EmailVerified bool       `db:"email_verified"`
	LockedUntil   *time.Time `db:"locked_until"`
	LastLoginAt   *time.Time `db:"last_login_at"`
	LastLoginIP   *string    `db:"last_login_ip"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}
