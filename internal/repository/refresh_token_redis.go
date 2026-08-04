package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"task_crud_api/internal/model"
)

type RedisRefreshRepo struct {
	rdb *redis.Client
}

func NewRedisRefreshRepo(rdb *redis.Client) *RedisRefreshRepo {
	return &RedisRefreshRepo{rdb: rdb}
}

func refreshKey(hash string) string { return "refresh:" + hash }

func userSetKey(userID uuid.UUID) string { return "user:" + userID.String() + ":refresh" }

func (r *RedisRefreshRepo) Create(ctx context.Context, userID uuid.UUID, hash string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	pipe := r.rdb.TxPipeline()
	pipe.Set(ctx, refreshKey(hash), userID.String(), ttl)
	pipe.SAdd(ctx, userSetKey(userID), hash)
	pipe.Expire(ctx, userSetKey(userID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisRefreshRepo) ByHash(ctx context.Context, hash string) (model.RefreshToken, error) {
	val, err := r.rdb.Get(ctx, refreshKey(hash)).Result()
	if errors.Is(err, redis.Nil) {
		return model.RefreshToken{}, model.ErrUnauthorized
	}
	if err != nil {
		return model.RefreshToken{}, err
	}
	userID, err := uuid.Parse(val)
	if err != nil {
		return model.RefreshToken{}, model.ErrUnauthorized
	}
	return model.RefreshToken{UserID: userID, TokenHash: hash}, nil
}

func (r *RedisRefreshRepo) DeleteByHash(ctx context.Context, hash string) error {
	val, err := r.rdb.Get(ctx, refreshKey(hash)).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	pipe := r.rdb.TxPipeline()
	pipe.Del(ctx, refreshKey(hash))
	if userID, perr := uuid.Parse(val); perr == nil {
		pipe.SRem(ctx, userSetKey(userID), hash)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRefreshRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	setKey := userSetKey(userID)
	hashes, err := r.rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}
	pipe := r.rdb.TxPipeline()
	for _, h := range hashes {
		pipe.Del(ctx, refreshKey(h))
	}
	pipe.Del(ctx, setKey)
	_, err = pipe.Exec(ctx)
	return err
}
