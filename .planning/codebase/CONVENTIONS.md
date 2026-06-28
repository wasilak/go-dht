# Coding Conventions

**Analysis Date:** 2026-06-28

## Naming Patterns

**Files:**
- `snake_case` not used — files are short, flat, and named by role: `main.go`, `commit.go`
- One binary target, single `package main`

**Functions:**
- `camelCase` for all functions: `recordMetrics`, `dhtSetup`, `dhtRun`, `runServer`, `initConfig`
- Constructor-style functions prefixed with `new`: `newRootCmd`, `newVersionCmd`
- Cobra `RunE` handler named `runServer` (returns error)

**Variables:**
- `camelCase` throughout: `tempGauge`, `humGauge`, `extendedLabels`, `labelNames`, `dhtInstance`
- Short names for loop vars and error returns: `err`, `ok`, `cmd`, `ctx`

**Types:**
- `PascalCase` structs: `sensorConfig`
- Struct tags use `mapstructure` for viper unmarshalling: `mapstructure:"id"`, `mapstructure:"pin"`, `mapstructure:"model"`

**Constants / Package-level vars:**
- `version` — unexported, injected at link time via `-ldflags`
- `Commit` — exported, computed at init from `debug.ReadBuildInfo()` in `commit.go`

## Error Handling

**Pattern:** explicit `if err != nil` everywhere, no panic except `prometheus.MustRegister` (intentional — misconfiguration should crash).

**Wrapping:** `fmt.Errorf("context: %s", err)` for wrapping (not `%w` — no unwrapping needed in this codebase).

**Fatal errors at startup:** `os.Exit(1)` used in `initConfig` for unreadable config files; cobra `RunE` propagates errors up to `main`.

**Sensor-level errors:** non-fatal — logged with `slog.Error` and `continue`; the loop keeps running for other sensors.

**Read errors in goroutine:** logged with `slog.Error` and loop continues; no backoff strategy.

## Logging

**Framework:** `log/slog` (stdlib) as the logging interface, initialized via `github.com/wasilak/loggergo`.

**Init pattern:**
```go
loggerConfig := loggergoTypes.Config{
    Level:  loggergoLib.LogLevelFromString(viper.GetString("loglevel")),
    Format: loggergoLib.LogFormatFromString(viper.GetString("logformat")),
}
loggergo.Init(ctx, loggerConfig, slog.Int("pid", os.Getpid()), slog.String("go_version", buildInfo.GoVersion))
```

**Usage pattern:** package-level `slog.*` calls (not a passed logger):
- `slog.Error(msg, slog.String("sensor", sensor.ID))` — structured key-value attrs
- `slog.Debug(fmt.Sprintf("temperature: %v, humidity: %v", ...)` — debug uses Sprintf (inconsistent with structured style — prefer `slog.Debug("...", slog.Float64(...))`)

**Log levels:** controlled via `--loglevel` flag / `GO_DHT_LOGLEVEL` env var. Default: `info`.

**Log format:** controlled via `--logformat` flag / `GO_DHT_LOGFORMAT` env var. Default: `json`.

**OpenTelemetry:** `loggergo` bridges slog to OTel (`go.opentelemetry.io/contrib/bridges/otelslog`) — logs are exportable via OTLP. `otelgo` (`github.com/wasilak/otelgo`) is an indirect dependency; OTel exporters (grpc/http) are present in `go.mod`.

## Config Conventions

**Stack:** cobra flags + viper + env vars.

**Binding:** `viper.BindPFlags(cmd.Flags())` binds all cobra flags to viper at root cmd creation.

**Env prefix:** `GO_DHT` — set in `main()` via `viper.SetEnvPrefix("GO_DHT")`.

**Env key mapping:** hyphens in flag names become underscores in env vars via `viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))`.

**Flag → viper key → env var mapping:**

| Flag | Viper key | Env var |
|------|-----------|---------|
| `--listen` | `listen` | `GO_DHT_LISTEN` |
| `--loglevel` | `loglevel` | `GO_DHT_LOGLEVEL` |
| `--logformat` | `logformat` | `GO_DHT_LOGFORMAT` |
| `--extended-labels` | `extended-labels` | `GO_DHT_EXTENDED_LABELS` |
| `--config` | n/a (persistent flag, not bound) | — |

**Config file:** searched as `go-dht.yaml` in `.`, `$HOME`, `/etc/go-dht/`. Override with `--config`.

**Sensors config:** YAML-only, not expressible as flags. Unmarshalled via `viper.UnmarshalKey("sensors", &sensors)`:
```yaml
sensors:
  - id: living-room
    pin: "4"
    model: DHT22
```

**Config file not found:** silently ignored. Other read errors → `os.Exit(1)`.

## Commit Message Style

**Format:** `type: description` (lowercase, no scope, no ticket reference).

**Types used:**
- `feat:` — new features
- `refactor:` — code restructuring without behavior change
- `Update ...` — dependency bumps (Renovate bot, no conventional prefix)

**Examples:**
```
feat: structured YAML sensor list instead of string format
feat: add config file support via viper
refactor: replace koanf with cobra + viper
feat: add multi-sensor support with per-sensor Prometheus label
Update softprops/action-gh-release action to v3 (#56)
```

Renovate PRs follow GitHub's default merge commit title (`Update module X to vY (#N)`), not conventional commits.

## Module Structure

**Single-package binary:** all code in `package main`, two files (`main.go`, `commit.go`).

**No internal packages:** the project is small enough that no `internal/` or sub-package structure is used.

---

*Convention analysis: 2026-06-28*
