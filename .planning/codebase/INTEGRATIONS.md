# External Integrations

**Analysis Date:** 2026-06-28

## Hardware Interfaces

**DHT11/DHT22 Temperature & Humidity Sensors:**
- Library: `github.com/prokopparuzek/go-dht` v0.1.1
- Underlying HAL: `periph.io/x/host/v3` (GPIO host initialization), `periph.io/x/conn/v3` (peripheral connection)
- GPIO pin addressed as `GPIO<N>` where `<N>` comes from sensor config `pin` field
- Temperature unit: Celsius (hardcoded in `dhtSetup()` at `main.go:55`)
- Read retry count: 10 attempts per read cycle (`main.go:36`)
- Poll interval: 10 seconds between reads (`main.go:49`)
- Multiple sensors supported — each runs in its own goroutine
- Requires root privileges for GPIO access

## Prometheus Metrics Exposed

**HTTP endpoint:** `/metrics` on configurable address (default `:9877`)
**Handler:** `promhttp.Handler()` — standard Prometheus exposition format

**Custom gauges registered in `main.go`:**

| Metric Name | Type | Help | Labels |
|---|---|---|---|
| `sensor_temperature` | GaugeVec | temperature from sensor | `sensor` (always); `model`, `pin` (if `--extended-labels`) |
| `sensor_humidity` | GaugeVec | humidity from sensor | `sensor` (always); `model`, `pin` (if `--extended-labels`) |

**Label dimensions:**
- `sensor` — sensor ID from config (always present)
- `model` — sensor model (e.g. `DHT22`) — only with `--extended-labels` flag or `GO_DHT_EXTENDED_LABELS=true`
- `pin` — GPIO pin number — only with `--extended-labels` flag

**Default Go runtime metrics** are also exposed (via default Prometheus registry): GC, goroutines, memory, process stats.

**Prometheus scrape config example:**
```yaml
scrape_configs:
  - job_name: go-dht
    static_configs:
      - targets: ['<host>:9877']
```

## Systemd Integration

**Unit file:** `go-dht.service`

**Service configuration:**
- Description: `DHT11/22 prometheus exporter`
- Requires: `network-online.target` (waits for network before starting)
- ExecStart: `/usr/local/bin/go-dht --config /etc/go-dht/go-dht.yaml`
- User/Group: `root` (required for GPIO hardware access)
- KillMode: `process`
- Restart policy: `on-failure`
- WantedBy: `multi-user.target`

**Service management:**
```bash
systemctl enable go-dht
systemctl start go-dht
systemctl status go-dht
journalctl -u go-dht -f
```

## Observability & Logging

**Logging framework:** `github.com/wasilak/loggergo` v1.8.2 — wraps `log/slog`

**Log fields injected at startup:**
- `pid` — process ID
- `go_version` — Go runtime version (from `debug.ReadBuildInfo()`)

**Log formats:** `json` (default) or `text` — set via `--logformat` / `GO_DHT_LOGFORMAT`

**Log levels:** `debug`, `info`, `warn`, `error` — set via `--loglevel` / `GO_DHT_LOGLEVEL`

**OpenTelemetry (indirect):**
- `github.com/wasilak/otelgo` v1.3.0 is a dependency of `loggergo` — pulls in full OTLP export capability
- OTLP exporters present for logs via gRPC and HTTP
- Not directly configured in `main.go` — activation depends on `loggergo` config and environment variables

## External APIs & Services

**None** — the application has no outbound HTTP calls to external APIs. All data flows inbound (sensor reads) and outbound only to Prometheus scrapers pulling `/metrics`.

## CI/CD External Services

**GitHub Actions** — CI/CD platform (`.github/workflows/`)

**Renovate Bot** — dependency update automation (`renovate.json`)
- Config schema: `https://docs.renovatebot.com/renovate-schema.json`
- Automerges patch and minor updates for Go modules
- Labels PRs with `renovate::dependencies` and type metadata

**Spectral (SpectralOps)** — secrets and misconfiguration scanning
- GitHub Action: `spectralops/spectral-github-action@v5`
- Auth: `SPECTRAL_DSN` secret in GitHub repository secrets
- Runs on push to `main`, tags, and PRs (`.github/workflows/scans.yml`)

**CodeQL** — GitHub's static analysis for security vulnerabilities
- Workflow: `.github/workflows/codeql-analysis.yml`

**CodeClimate** — maintainability badge shown in `README.md`
- Badge URL: `https://api.codeclimate.com/v1/badges/b1a1245e15f788148b03/maintainability`
- Read-only integration (badge only, no gate)

**softprops/action-gh-release** — GitHub Release creation on tag push
- Uploads ZIP artifacts per platform
- Auto-generates release notes from git history

## Environment Variables

| Variable | Maps To | Default | Description |
|---|---|---|---|
| `GO_DHT_LISTEN` | `--listen` | `:9877` | HTTP listen address |
| `GO_DHT_LOGLEVEL` | `--loglevel` | `info` | Log level |
| `GO_DHT_LOGFORMAT` | `--logformat` | `json` | Log format |
| `GO_DHT_EXTENDED_LABELS` | `--extended-labels` | `false` | Add model/pin labels to metrics |
| `GO_DHT_CONFIG` | `--config` | `` | Explicit config file path |

---

*Integration audit: 2026-06-28*
