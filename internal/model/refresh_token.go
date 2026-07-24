package model

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken is a stored refresh-token record. Only the hash is kept, never
// the token itself, so this struct is internal and never serialized to a client.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}
