<!-- refreshed: 2026-06-28 -->
# Architecture

**Analysis Date:** 2026-06-28

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                    CLI (cobra)  main()                       │
│  `main.go` — flags, env prefix, cobra.OnInitialize          │
└────────────────────┬────────────────────────────────────────┘
                     │ runServer()
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              Config Resolution (viper)                       │
│  CLI flags → env vars → config file → defaults              │
│  `main.go:initConfig`, `main.go:runServer`                  │
└──────────┬────────────────────────┬───────────────────────┘
           │                        │
           ▼                        ▼
┌─────────────────────┐   ┌─────────────────────────────────┐
│  Sensor goroutines  │   │  Prometheus registry             │
│  one per sensor     │   │  sensor_temperature GaugeVec     │
│  `recordMetrics()`  │   │  sensor_humidity GaugeVec        │
│  reads every 10s    │   │  labels: sensor [+ model, pin]   │
└──────┬──────────────┘   └───────────────┬─────────────────┘
       │ DHT hardware read                │
       ▼                                  ▼
┌─────────────────────┐   ┌─────────────────────────────────┐
│  periph.io DHT lib  │   │  HTTP /metrics (promhttp)        │
│  GPIO access        │   │  `main.go:runServer`             │
│  `dhtSetup/dhtRun`  │   │  listens on :9877 by default     │
└─────────────────────┘   └─────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| `main()` | Wire cobra, viper env prefix, execute root command | `main.go:183` |
| `newRootCmd()` | Declare CLI flags, bind to viper | `main.go:79` |
| `initConfig()` | Resolve config file path and load via viper | `main.go:164` |
| `runServer()` | Orchestrate: parse sensors config, create gauges, spawn goroutines, start HTTP | `main.go:107` |
| `dhtSetup()` | Initialize periph.io host and DHT device for a single sensor | `main.go:54` |
| `dhtRun()` | Read humidity+temperature from DHT with retry | `main.go:68` |
| `recordMetrics()` | Spawn goroutine: infinite loop reading sensor and updating gauges | `main.go:32` |
| `Commit` (package-level var) | Extract VCS revision from build info at init time | `commit.go:5` |

## Config Loading Chain

```
cobra flags (--listen, --loglevel, --logformat, --extended-labels, --config)
    │
    ▼ viper.BindPFlags()  [main.go:92]
viper flag values
    │
    ▼ viper.AutomaticEnv() + SetEnvPrefix("GO_DHT")  [main.go:184-185]
env vars: GO_DHT_LISTEN, GO_DHT_LOGLEVEL, GO_DHT_LOGFORMAT, GO_DHT_EXTENDED_LABELS
    │
    ▼ initConfig() → viper.ReadInConfig()  [main.go:175]
config file (YAML):
  search order: ./go-dht.yaml → $HOME/go-dht.yaml → /etc/go-dht/go-dht.yaml
  or --config <explicit path>
    │
    ▼ viper defaults (cobra flag defaults act as viper defaults)
:9877 / info / json / false
```

Key: `sensors` list is config-file-only — no CLI flag or env var maps to it.
Env key replacer converts `-` → `_` so `extended-labels` maps to `GO_DHT_EXTENDED_LABELS`.

## Concurrency Model

One goroutine is spawned per successfully initialized sensor via `recordMetrics()`.
Each goroutine runs an infinite `for` loop with a fixed `time.Sleep(10 * time.Second)` poll interval.
There is no channel, WaitGroup, or context cancellation — goroutines run for the lifetime of the process.
The main goroutine blocks on `http.ListenAndServe`.

```
main goroutine ──► http.ListenAndServe (blocks)
sensor[0] goroutine ──► dhtRun → gauge.Set → sleep(10s) → repeat
sensor[1] goroutine ──► dhtRun → gauge.Set → sleep(10s) → repeat
...
```

Read errors are logged (slog.Error) but do not stop the loop; stale gauge values persist until the next successful read.

## Prometheus Metrics Design

**Metrics registered:**
- `sensor_temperature` (GaugeVec) — current temperature in Celsius
- `sensor_humidity` (GaugeVec) — current relative humidity

**Label strategy — two modes:**

| Mode | Labels | Flag |
|------|--------|------|
| Default | `sensor` only | (omit `--extended-labels`) |
| Extended | `sensor`, `model`, `pin` | `--extended-labels` |

The `labelNames` slice is determined once at startup and passed into `NewGaugeVec`. Label cardinality is fixed for the process lifetime.

`sensor` value = `id` field from sensor config (user-defined string, e.g. `bedroom`).
`model` value = DHT model string (e.g. `DHT22`).
`pin` value = GPIO pin number string (e.g. `4`).

Metrics are exposed at `GET /metrics` via `promhttp.Handler()`.

## Error Handling

**Strategy:** Log and continue.

- `dhtSetup` failure for a sensor: log error, skip that sensor (`continue`), proceed with others.
- `dhtRun` failure in goroutine: log error, continue loop; gauge retains last value.
- Config file not found: tolerated (viper.ConfigFileNotFoundError silently ignored).
- Config file found but malformed: fatal — prints to stderr and `os.Exit(1)`.
- No sensors configured: `runServer` returns error → cobra prints it and exits non-zero.

## Deployment

Runs as a systemd service (`go-dht.service`) under root (required for GPIO access).
Config path at `/etc/go-dht/go-dht.yaml`.
Binary at `/usr/local/bin/go-dht`.
Restart policy: `on-failure`.

---

*Architecture analysis: 2026-06-28*
