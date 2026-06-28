# Codebase Structure

**Analysis Date:** 2026-06-28

## Directory Layout

```
go-dht/
├── main.go              # All application logic (single file)
├── commit.go            # Package-level VCS revision extraction
├── go.mod               # Module: github.com/wasilak/go-dht, Go 1.25.4
├── go.sum               # Dependency checksums
├── go-dht.service       # systemd unit file for production deployment
├── go-dht               # Compiled binary (committed, 25 MB)
├── renovate.json        # Renovate bot config for dependency updates
├── README.md            # Project readme
├── .github/             # GitHub Actions workflows
├── .planning/           # GSD planning documents
└── .gitignore
```

## Key Types

**`sensorConfig`** (`main.go:26`):
```go
type sensorConfig struct {
    ID    string `mapstructure:"id"`
    Pin   string `mapstructure:"pin"`
    Model string `mapstructure:"model"`
}
```
Populated by `viper.UnmarshalKey("sensors", &sensors)` from the YAML config `sensors` list.

## Key Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `main` | `func main()` | Entry point — sets up viper env handling, builds cobra tree, executes |
| `newRootCmd` | `func newRootCmd() *cobra.Command` | Declares all CLI flags, binds them to viper |
| `newVersionCmd` | `func newVersionCmd() *cobra.Command` | Prints version string (set at link time via `-ldflags`) |
| `initConfig` | `func initConfig(cmd *cobra.Command)` | Resolves and reads config file via viper |
| `runServer` | `func runServer(cmd *cobra.Command, args []string) error` | Main business logic: init logger, parse sensors, register metrics, spawn goroutines, serve HTTP |
| `dhtSetup` | `func dhtSetup(pin string, model string) (*dht.DHT, error)` | Initializes periph.io host and DHT sensor instance |
| `dhtRun` | `func dhtRun(dht *dht.DHT, retry int) (float64, float64, error)` | Reads sensor with retry; returns (humidity, temperature, error) |
| `recordMetrics` | `func recordMetrics(dhtInstance *dht.DHT, sensor sensorConfig, tempGauge, humGauge *prometheus.GaugeVec, extendedLabels bool)` | Spawns goroutine that polls sensor every 10s and updates gauges |

**`Commit`** (`commit.go:5`):
Package-level `var` initialized via IIFE. Reads `vcs.revision` from `debug.BuildInfo`. Used for version reporting.

**`version`** (`main.go:24`):
Package-level `var string`, injected at build time via `-ldflags "-X main.version=..."`.

## Entry Points

**Binary execution:**
1. `main()` → sets `GO_DHT` env prefix on viper, registers env-to-flag key replacer (`-` → `_`), calls `viper.AutomaticEnv()`
2. Builds cobra root command via `newRootCmd()`, adds `version` subcommand
3. Registers `cobra.OnInitialize(func() { initConfig(root) })` — runs before any command
4. `root.Execute()` → parses args → calls `runServer` (default) or prints version

**`runServer` execution flow:**
1. Initialize structured logger (loggergo/slog) with pid and Go version in base fields
2. Unmarshal `sensors` from viper config into `[]sensorConfig`
3. Determine label names based on `--extended-labels` flag
4. Register `sensor_temperature` and `sensor_humidity` GaugeVecs with Prometheus default registry
5. For each sensor: call `dhtSetup`, on success call `recordMetrics` (spawns goroutine)
6. Register `promhttp.Handler()` at `/metrics`
7. `http.ListenAndServe` blocks until process exit

## Configuration File Format (YAML)

```yaml
# Required — no CLI/env equivalent
sensors:
  - id: bedroom
    pin: "4"
    model: DHT22

# Optional — all have CLI flags and GO_DHT_* env var equivalents
listen: ":9877"
loglevel: info
logformat: json
extended-labels: false
```

## External Dependencies (direct)

| Package | Version | Role |
|---------|---------|------|
| `github.com/prokopparuzek/go-dht` | v0.1.1 | DHT11/22 hardware driver (periph.io based) |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics and HTTP handler |
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `github.com/spf13/viper` | v1.21.0 | Config management (flags, env, files) |
| `github.com/wasilak/loggergo` | v1.8.2 | slog-based structured logger with OTel support |

## Where to Add New Code

**New CLI flag:** Add to `newRootCmd()` via `cmd.Flags()` then `viper.BindPFlags` already covers it.

**New metric:** Define additional `GaugeVec` (or `CounterVec`) in `runServer()`, register with `prometheus.MustRegister`, pass into `recordMetrics` or a new recording function.

**New sensor driver:** Add a new setup/run function pair following the `dhtSetup`/`dhtRun` pattern; dispatch in `runServer` based on `sensor.Model`.

**New subcommand:** Create `newXxxCmd()` returning `*cobra.Command`, add with `root.AddCommand(newXxxCmd())` in `main()`.

All new code goes in `main.go` (this is a single-file application) unless it warrants its own file for clarity (follow `commit.go` as the pattern for isolated package-level concerns).

---

*Structure analysis: 2026-06-28*
