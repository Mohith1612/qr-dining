//go:build integration

package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

func TestWSTicketStoreConsumeOnce(t *testing.T) {
	client := openTestRedis(t)
	store := redisPkg.NewWSTicketStore(client)
	ctx := context.Background()

	ticket, err := store.Issue(ctx, testTicketClaims())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := store.Consume(ctx, ticket); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	if _, err := store.Consume(ctx, ticket); err == nil {
		t.Fatal("second Consume succeeded, want invalid ticket")
	}
}

func TestWSTicketStoreExpiredTicketRejected(t *testing.T) {
	client := openTestRedis(t)
	store := redisPkg.NewWSTicketStoreWithTTL(client, 10*time.Millisecond)
	ctx := context.Background()

	ticket, err := store.Issue(ctx, testTicketClaims())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := store.Consume(ctx, ticket); err == nil {
		t.Fatal("Consume succeeded for expired ticket")
	}
}

func testTicketClaims() redisPkg.WSTicketClaims {
	return redisPkg.WSTicketClaims{
		SessionID:         uuid.New(),
		OrganizationID:    1,
		BranchID:          2,
		ParticipantID:     3,
		CredentialVersion: 1,
		JTI:               uuid.NewString(),
	}
}

func openTestRedis(t *testing.T) *goredis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse TEST_REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	return client
}
