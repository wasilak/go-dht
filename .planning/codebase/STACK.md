# Technology Stack

**Analysis Date:** 2026-06-28

## Languages

**Primary:**
- Go 1.25.4 — entire application (`main.go`, `go.mod`)

## Runtime

**Environment:**
- Go runtime (no separate VM/interpreter)

**Package Manager:**
- Go modules (`go mod`)
- Lockfile: `go.sum` present

## Frameworks

**CLI:**
- `github.com/spf13/cobra` v1.10.2 — CLI command structure and flag parsing
- `github.com/spf13/viper` v1.21.0 — configuration loading (YAML, env vars, flags)

**Observability:**
- `github.com/prometheus/client_golang` v1.23.2 — Prometheus metrics exposition
- `github.com/wasilak/loggergo` v1.8.2 — structured logging via `log/slog`
- `github.com/wasilak/otelgo` v1.3.0 (indirect) — OpenTelemetry integration
- OpenTelemetry SDK v1.44.0 (indirect) — traces, metrics, logs via OTLP

**Hardware:**
- `github.com/prokopparuzek/go-dht` v0.1.1 — DHT11/DHT22 sensor reads via GPIO
- `periph.io/x/conn/v3` v3.7.3 (indirect) — hardware peripheral connectivity
- `periph.io/x/host/v3` v3.8.5 (indirect) — host hardware abstraction (GPIO init)

**Testing:**
- Not detected — no test files present

**Build/Dev:**
- Standard `go build` — no Makefile or build scripts detected
- `-ldflags "-X main.version=..."` injects version at build time from git tag

## Key Dependencies

**Critical:**
- `github.com/prokopparuzek/go-dht` v0.1.1 — reads temperature and humidity from DHT sensors over GPIO; without this the binary has no sensor capability
- `github.com/prometheus/client_golang` v1.23.2 — exposes `/metrics` HTTP endpoint consumed by Prometheus scrapers
- `github.com/spf13/viper` v1.21.0 — merges config file + env vars + CLI flags into unified config

**Infrastructure:**
- `github.com/cenkalti/backoff/v5` v5.0.3 — retry/backoff (indirect, likely from loggergo/otelgo)
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` v0.20.0 — OTLP log export over gRPC
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` v0.20.0 — OTLP log export over HTTP

## Configuration

**Environment:**
- Env var prefix: `GO_DHT_` (e.g. `GO_DHT_LISTEN`, `GO_DHT_LOGLEVEL`)
- Hyphens in flag names map to underscores in env vars
- Config file search order: `./go-dht.yaml`, `~/.go-dht.yaml`, `/etc/go-dht/go-dht.yaml`
- Config file specified explicitly via `--config` flag

**Key config keys:**
- `sensors` — list of sensor objects with `id`, `pin`, `model` fields
- `listen` — HTTP listen address (default `:9877`)
- `loglevel` — `debug | info | warn | error` (default `info`)
- `logformat` — `json | text` (default `json`)
- `extended-labels` — bool, adds `model` and `pin` as Prometheus label dimensions

**Build:**
- No dedicated build config file — CI uses inline `go build` commands
- Version injected at link time: `go build -ldflags "-X main.version=<git-tag>"`

## CI/CD

**Platform:** GitHub Actions

**Workflows:**
- `.github/workflows/main.yml` — CI build and release
  - Triggers: push to `main`, any tag, pull requests to `main`, manual dispatch
  - Matrix: `linux/amd64`, `linux/arm64`, `linux/arm` (GOARM 5/6/7), `darwin/amd64`, `darwin/arm64`
  - Artifacts: ZIP archives per platform in `./dist/`
  - Release: `softprops/action-gh-release@v3` uploads ZIPs on tag push with auto-generated release notes
- `.github/workflows/scans.yml` — security scanning
  - Spectral secret/misconfiguration scan (`spectralops/spectral-github-action@v5`)
  - Snyk and Codacy scans present but commented out
- `.github/workflows/codeql-analysis.yml` — CodeQL static analysis
- `.github/workflows/stale.yml` — stale issue/PR management

**Dependency Updates:**
- Renovate Bot (`renovate.json`) — auto-merges patch/minor updates, runs `go mod tidy` post-update, labels PRs with type/datasource metadata, separates major from minor updates

## Platform Requirements

**Development:**
- Go 1.25.4+
- Linux/macOS (GPIO access requires Linux on actual hardware)

**Production:**
- Linux with GPIO access (Raspberry Pi or similar ARM SBC)
- Binary installed at `/usr/local/bin/go-dht`
- Config at `/etc/go-dht/go-dht.yaml`
- Runs as `root` (required for GPIO hardware access)
- Managed by systemd (`go-dht.service`)

## Deployment Artifacts

- Single statically-linked binary `go-dht`
- Distributed as ZIP archives per platform via GitHub Releases
- Systemd unit file: `go-dht.service`

---

*Stack analysis: 2026-06-28*
