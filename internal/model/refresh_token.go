package model

import "time"

// RefreshToken is a stored refresh-token record. Only the hash is kept, never
// the token itself, so this struct is internal and never serialized to a client.
type RefreshToken struct {
	ID        int
	UserID    int
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}
