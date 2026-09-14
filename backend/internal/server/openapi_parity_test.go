package server

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/Mohith1612/qr-dining/internal/events"
	"github.com/Mohith1612/qr-dining/internal/observability"
	redisPkg "github.com/Mohith1612/qr-dining/internal/redis"
	"github.com/Mohith1612/qr-dining/internal/repository"
	ws "github.com/Mohith1612/qr-dining/internal/websocket"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

var ginParameter = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

var openAPIMethods = map[string]struct{}{
	"delete":  {},
	"get":     {},
	"head":    {},
	"options": {},
	"patch":   {},
	"post":    {},
	"put":     {},
	"trace":   {},
}

func TestRouterOpenAPIParity(t *testing.T) {
	registered := registeredRouteMethods(t)
	documented := documentedRouteMethods(t)

	paths := make(map[string]struct{}, len(registered)+len(documented))
	for path := range registered {
		paths[path] = struct{}{}
	}
	for path := range documented {
		paths[path] = struct{}{}
	}

	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)

	var mismatches []string
	for _, path := range orderedPaths {
		routerMethods, inRouter := registered[path]
		specMethods, inSpec := documented[path]
		switch {
		case !inSpec:
			mismatches = append(mismatches, "registered path absent from OpenAPI: "+path+" "+strings.Join(routerMethods, ","))
		case !inRouter:
			mismatches = append(mismatches, "documented path not registered: "+path+" "+strings.Join(specMethods, ","))
		case !reflect.DeepEqual(routerMethods, specMethods):
			mismatches = append(mismatches, "method mismatch for "+path+": router="+strings.Join(routerMethods, ",")+" openapi="+strings.Join(specMethods, ","))
		}
	}

	if len(mismatches) > 0 {
		t.Fatalf("router/OpenAPI parity failed:\n  %s", strings.Join(mismatches, "\n  "))
	}
}

func registeredRouteMethods(t *testing.T) map[string][]string {
	t.Helper()

	logger := zerolog.Nop()
	metrics := observability.NewMetrics()
	redisClient := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = redisClient.Close() })
	pubsub := redisPkg.NewPubSub(redisClient, logger, metrics)
	publisher := events.NewPublisher(pubsub, logger)
	hub := ws.NewHub(pubsub, metrics, logger, nil)
	repos := repository.New(nil, logger)
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode:          gin.TestMode,
			RateLimitRPM:     60,
			AuthRateLimitRPM: 10,
		},
		Auth: config.AuthConfig{
			GuestTokenSecret: "router-openapi-parity-test-secret",
			GuestTokenTTL:    time.Hour,
		},
		Payment: config.PaymentConfig{
			WebhookSecrets:            map[string]string{},
			WebhookTimestampTolerance: time.Minute,
		},
		Presence: config.PresenceConfig{HostAbsenceGrace: time.Minute},
	}

	routes := New(cfg, nil, redisClient, hub, metrics, logger, repos, publisher).router.Routes()
	result := make(map[string][]string)
	for _, route := range routes {
		path := ginParameter.ReplaceAllString(route.Path, `{$1}`)
		result[path] = append(result[path], strings.ToUpper(route.Method))
	}
	for path := range result {
		sort.Strings(result[path])
	}
	return result
}

func documentedRouteMethods(t *testing.T) map[string][]string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate parity test source")
	}
	specPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "openapi.yaml")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}

	var document struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}

	result := make(map[string][]string, len(document.Paths))
	for path, item := range document.Paths {
		for key := range item {
			method := strings.ToLower(key)
			if _, ok := openAPIMethods[method]; ok {
				result[path] = append(result[path], strings.ToUpper(method))
			}
		}
		sort.Strings(result[path])
	}
	return result
}
