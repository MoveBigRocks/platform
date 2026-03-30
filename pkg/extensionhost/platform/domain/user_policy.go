package platformdomain

import (
	"fmt"
	"time"
)

// NormalizeOperatorRole validates and canonicalizes an instance admin role.
func NormalizeOperatorRole(role *InstanceRole) (*InstanceRole, error) {
	if role == nil {
		return nil, nil
	}

	canonical := CanonicalizeInstanceRole(*role)
	if !canonical.IsOperator() {
		return nil, fmt.Errorf("invalid instance role: %s", *role)
	}
	return &canonical, nil
}

// NewManagedUser creates a user through the platform admin lifecycle rules.
func NewManagedUser(email, name string, instanceRole *InstanceRole, now time.Time) (*User, error) {
	canonicalRole, err := NormalizeOperatorRole(instanceRole)
	if err != nil {
		return nil, err
	}

	return &User{
		Email:        email,
		Name:         name,
		InstanceRole: canonicalRole,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// UpdateManagedProfile applies an instance-admin update to a user.
func (u *User) UpdateManagedProfile(
	email, name string,
	instanceRole *InstanceRole,
	isActive, emailVerified bool,
	updatedAt time.Time,
) error {
	canonicalRole, err := NormalizeOperatorRole(instanceRole)
	if err != nil {
		return err
	}

	u.Email = email
	u.Name = name
	u.InstanceRole = canonicalRole
	u.IsActive = isActive
	u.EmailVerified = emailVerified
	u.UpdatedAt = updatedAt
	return nil
}

// EnsureAnotherActiveSuperAdmin enforces the invariant that at least one active super admin remains.
func EnsureAnotherActiveSuperAdmin(users []*User, excludedUserID, action string) error {
	for _, user := range users {
		if user == nil || user.ID == excludedUserID {
			continue
		}
		if user.IsSuperAdmin() && user.IsActive {
			return nil
		}
	}

	return fmt.Errorf("cannot %s the last active super admin", action)
}
