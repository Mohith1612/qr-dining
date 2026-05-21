// Package redis wraps go-redis for pub/sub, presence tracking, rate limiting, and short-lived cache.
// Redis is never the source of truth — PostgreSQL owns all durable business state.
package redis
