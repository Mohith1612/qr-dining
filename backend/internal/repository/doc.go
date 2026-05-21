// Package repository provides thin wrappers over sqlc queries.
// Repositories translate pgx errors to domain errors and define interfaces for testing.
// Business logic must not live here — it belongs in services.
package repository
