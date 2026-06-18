package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const wsTicketTTL = 30 * time.Second

var ErrWSTicketInvalid = errors.New("websocket ticket invalid")

type WSTicketClaims struct {
	SessionID         uuid.UUID `json:"session_id"`
	OrganizationID    int64     `json:"organization_id"`
	BranchID          int64     `json:"branch_id"`
	ParticipantID     int64     `json:"participant_id"`
	CredentialVersion int32     `json:"credential_version"`
	JTI               string    `json:"jti"`
	IssuedAtUnix      int64     `json:"iat"`
	ExpiresAtUnix     int64     `json:"exp"`
}

type WSTicketStore struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewWSTicketStore(client *goredis.Client) *WSTicketStore {
	return &WSTicketStore{client: client, ttl: wsTicketTTL}
}

func NewWSTicketStoreWithTTL(client *goredis.Client, ttl time.Duration) *WSTicketStore {
	return &WSTicketStore{client: client, ttl: ttl}
}

func (s *WSTicketStore) Issue(ctx context.Context, claims WSTicketClaims) (string, error) {
	token, err := randomTicket()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims.IssuedAtUnix = now.Unix()
	claims.ExpiresAtUnix = now.Add(s.ttl).Unix()

	data, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal websocket ticket: %w", err)
	}
	if err := s.client.Set(ctx, wsTicketKey(token), data, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("store websocket ticket: %w", err)
	}
	return token, nil
}

func (s *WSTicketStore) Consume(ctx context.Context, token string) (WSTicketClaims, error) {
	if token == "" {
		return WSTicketClaims{}, ErrWSTicketInvalid
	}
	raw, err := s.client.GetDel(ctx, wsTicketKey(token)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return WSTicketClaims{}, ErrWSTicketInvalid
	}
	if err != nil {
		return WSTicketClaims{}, fmt.Errorf("consume websocket ticket: %w", err)
	}
	var claims WSTicketClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return WSTicketClaims{}, ErrWSTicketInvalid
	}
	if claims.ExpiresAtUnix <= time.Now().UTC().Unix() {
		return WSTicketClaims{}, ErrWSTicketInvalid
	}
	return claims, nil
}

func wsTicketKey(token string) string {
	return "ws_ticket:" + token
}

func randomTicket() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate websocket ticket: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
