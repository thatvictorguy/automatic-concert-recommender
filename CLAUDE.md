# Automatic Concert Recommender

## Overview
Automatically suggests upcoming concerts in Japan based on a user's Spotify listening habits and sends a digest to a private Discord server. Built in phases: CLI first, then HTTP API with UI, then Kubernetes deployment.

## Tech Stack
- **Language:** Go
- **CLI framework:** Cobra (Phase 1)
- **HTTP framework:** Gin (Phase 2)
- **External APIs:** Spotify Web API, Setlist.fm API
- **Notifications:** Discord webhooks (Phase 1 output delivery)
- **Containerization:** Docker + Kubernetes (Phase 3)

## Project Structure (DDD)
```
automatic-concert-recommender/
├── domain/              # Core business logic — no external dependencies
│   ├── artist.go
│   ├── concert.go
│   └── recommender.go
├── application/         # Use cases / orchestration
│   └── recommend.go
├── infrastructure/      # External integrations
│   ├── spotify/         # MusicSource: top artists via Spotify Web API
│   ├── setlistfm/       # ConcertFinder: upcoming Japan concerts via Setlist.fm
│   └── discord/         # Notifier: sends digest to Discord via webhook
└── interfaces/          # Delivery mechanisms
    ├── cli/             # Phase 1: Cobra CLI
    └── http/            # Phase 2: Gin HTTP handlers
```

## Development Phases
- **Phase 1:** CLI tool (Cobra) — fetch Spotify data, find Japan concerts via Setlist.fm, send Discord digest
- **Phase 2:** Add Gin HTTP API + UI (with configurable time ranges and concert window)
- **Phase 3:** Containerize and deploy to Kubernetes

## Commands
- Build: `go build ./...`
- Test: `go test ./...`
- Run CLI: `go run main.go`
- Lint: `golangci-lint run`

## Principles
- **Test-driven development (TDD):** Write tests before implementation. Every domain and application layer function must have tests.
- **Domain-driven design (DDD):** Keep domain logic pure and free of framework/infrastructure dependencies. Domain layer must not import from infrastructure or interfaces.
- **Simple but robust:** Prefer clarity over cleverness. No premature abstractions.

## Conventions
- Follow standard Go conventions (`gofmt`, idiomatic naming)
- Use interfaces for all external dependencies to enable mocking in tests
- Errors must be handled explicitly — no silent failures
- One package per directory, named after the directory
- Commit style: conventional commits (`feat:`, `fix:`, `test:`, `refactor:`)
