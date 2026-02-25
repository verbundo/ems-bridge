# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...          # build
go run .                # run (loads config.yml from cwd)
go test ./...           # run all tests
go test ./... -run TestFoo  # run a single test
```

## Architecture

`ems-bridge` is a routing bridge that moves messages between endpoints defined in `config.yml`.

**Config model** (`config.go`):
- `ConnectorConfig` — a named endpoint with `type`, `url`, `username`, `password`
- `RouteConfig` — a named route with a single `from` source and one or more `to` destinations
- `Config` — top-level struct; `routes` key maps to `[]RouteConfig`; all other top-level keys are parsed inline as `map[string]ConnectorConfig`

**Address format** (used in `from`/`to` fields): `<connector-name-or-scheme>:<details>`, e.g.:
- `fs:./data/in` — local filesystem path
- `ems-dev:queue:tmp.q` — named connector + queue path

`LoadConfig("config.yml")` is called in `main()` and is the entry point for all configuration.
