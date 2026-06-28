# Codebase Concerns

**Analysis Date:** 2026-06-28

---

## HIGH Priority

### Silent sensor setup failure — server starts with no active metrics

- Issue: In `runServer` (`main.go`), if `dhtSetup` fails for a sensor, the error is logged and the loop `continue`s. The HTTP server starts and `/metrics` is served with no data being collected for the failed sensor. There is no indication to Prometheus that the sensor is down — the gauge simply shows its last value (or nothing if it never succeeded).
- Files: `main.go` — `runServer` loop, `dhtSetup` function
- Impact: Silent data loss. Prometheus scrapes succeed with stale or zero values. No alertable signal.
- Fix approach: Return a fatal error if *all* sensors fail setup. For partial failure, expose a `sensor_up` gauge per sensor ID set to 0/1.

---

### Metrics set to 0,0 on read error — stale/misleading data

- Issue: In `recordMetrics` (`main.go`), when `dhtRun` returns an error, execution falls through and calls `tempGauge.With(labels).Set(temperature)` and `humGauge.With(labels).Set(humidity)` with the zero values returned on error. This makes 0°C / 0% indistinguishable from a failed read.
- Files: `main.go` — `recordMetrics` goroutine
- Impact: Alerts based on value thresholds will miss sensor failures entirely. A broken sensor looks identical to one reading 0.
- Fix approach: Skip the `Set` calls on error. Optionally use `prometheus.NewGaugeVec` with a companion `sensor_up` gauge to signal read health.

---

### Potential nil panic on `debug.ReadBuildInfo()` failure

- Issue: `buildInfo, _ := debug.ReadBuildInfo()` discards the `ok` boolean. The immediately following `loggergo.Init(ctx, loggerConfig, ..., slog.String("go_version", buildInfo.GoVersion))` will panic with a nil pointer dereference if `ReadBuildInfo` returns `(nil, false)` — which can happen when the binary is not built with module support.
- Files: `main.go` — `runServer`
- Impact: Process crashes at startup in non-module builds or stripped binaries.
- Fix approach: Guard with `if buildInfo != nil` before accessing `buildInfo.GoVersion`.

---

### Runs as root with no privilege drop

- Issue: `go-dht.service` sets `User=root` / `Group=root`. GPIO access on Linux requires elevated privileges or membership in the `gpio` group (kernel ≥ 4.8 with character device GPIO).
- Files: `go-dht.service`
- Impact: A compromised or buggy process has full root access to the host. Violates principle of least privilege.
- Fix approach: Create a dedicated `go-dht` system user, add it to the `gpio` group, and switch `User=go-dht` / `Group=gpio`. Add `NoNewPrivileges=true`, `ProtectSystem=strict`, `PrivateTmp=true` to the unit file.

---

### No tests — entire codebase is untested

- Issue: No `*_test.go` files exist anywhere in the repository. There are no unit tests, integration tests, or table-driven tests of any kind.
- Files: entire repo
- Impact: Any refactoring or dependency upgrade may silently break behavior. CI only verifies that the binary compiles.
- Fix approach: At minimum, unit-test `dhtRun` error path, `dhtSetup` error propagation, and config parsing via `viper.UnmarshalKey`. Use the `prokopparuzek/go-dht` interface for mocking.

---

## MEDIUM Priority

### `dht.HostInit()` called once per sensor

- Issue: `dhtSetup` calls `dht.HostInit()` for every configured sensor. `HostInit` (from `periph.io/x/host`) is designed to be called once per process; multiple calls are technically idempotent but wasteful and may mask initialization errors in subsequent sensors.
- Files: `main.go` — `dhtSetup`
- Impact: Minor — currently harmless, but masks the intent and couples setup logic incorrectly.
- Fix approach: Call `dht.HostInit()` once at the top of `runServer` before the sensor loop. Fail fast if it returns an error.

---

### No graceful shutdown

- Issue: `http.ListenAndServe` blocks without a shutdown context. There is no `os.Signal` handler. Systemd sends `SIGTERM` on stop (`KillMode=process`), which kills the process immediately without draining in-flight scrapes.
- Files: `main.go` — `runServer`
- Impact: In-flight Prometheus scrapes are dropped on restart. Not critical for an exporter, but clean shutdown is a baseline operational expectation.
- Fix approach: Use `http.Server` with `Shutdown(ctx)`, listen for `SIGTERM`/`SIGINT` via `signal.NotifyContext`, and call `Shutdown` with a short deadline (e.g. 5s).

---

### No health or liveness endpoint

- Issue: The only HTTP route is `/metrics`. There is no `/healthz` or `/readyz` endpoint. Kubernetes probes or load balancers cannot distinguish a healthy exporter from a crashed one (beyond TCP connect).
- Files: `main.go` — `runServer`
- Impact: Operational observability gap. Cannot probe liveness independently of metrics scraping.
- Fix approach: Add a trivial `http.HandleFunc("/healthz", ...)` that returns 200 OK.

---

### No config reload support

- Issue: Viper ships with `fsnotify`-based config watching (`viper.WatchConfig`), but it is not wired up. Config changes require a full process restart.
- Files: `main.go` — `initConfig`, `runServer`
- Impact: Adding or removing sensors requires downtime. On a Raspberry Pi this is a manual operation.
- Fix approach: Wire `viper.OnConfigChange` + `viper.WatchConfig` and re-initialize sensor goroutines on change, or document that restart is required and make it explicit in the README.

---

### Fixed 10-second polling with no backoff on repeated errors

- Issue: `recordMetrics` sleeps for exactly 10 seconds between reads regardless of error state. A persistently failing sensor logs an error every 10 seconds indefinitely.
- Files: `main.go` — `recordMetrics` goroutine
- Impact: Log noise; no exponential backoff means no natural dampening of transient hardware faults.
- Fix approach: Implement simple exponential backoff (e.g. up to 5-minute ceiling) on consecutive errors, resetting on successful read.

---

### `viper.BindPFlags` error ignored

- Issue: `viper.BindPFlags(cmd.Flags())` returns an error that is silently discarded in `newRootCmd`.
- Files: `main.go` — `newRootCmd`
- Impact: Misconfigured flag binding would fail silently, causing flags to have no effect.
- Fix approach: Check the error: `if err := viper.BindPFlags(cmd.Flags()); err != nil { panic(err) }` (acceptable at init time).

---

### `prokopparuzek/go-dht` — low-activity upstream

- Issue: `github.com/prokopparuzek/go-dht` is at `v0.1.1` with minimal upstream activity. It is the only library providing actual hardware communication.
- Files: `go.mod`
- Impact: Security patches or GPIO API changes in `periph.io` may not be reflected upstream; no interface abstraction makes it hard to swap.
- Fix approach: Wrap the DHT calls behind a local `Sensor` interface to allow future substitution. Monitor upstream for activity.

---

## LOW Priority

### Compiled binary committed to the repository

- Issue: `go-dht` (25.4 MB ELF binary) is committed at the repo root.
- Files: `go-dht` (binary)
- Impact: Inflates repo clone size; binary may be stale relative to source; creates confusion about which artifact to deploy.
- Fix approach: Add `go-dht` (the binary, without extension) to `.gitignore`. Use GitHub Releases (already configured in CI) as the canonical distribution artifact.

---

### `go 1.25.4` in `go.mod` — future version directive

- Issue: `go.mod` declares `go 1.25.4`. As of this analysis Go 1.24.x is the latest stable release. This directive may have been set speculatively or by a toolchain managing the file.
- Files: `go.mod`
- Impact: May cause `go build` to fail on any system running a stable Go toolchain older than 1.25. CI uses `go-version-file: go.mod` so it will attempt to download 1.25.4, which may not exist.
- Fix approach: Verify the actual minimum required Go version and update `go.mod` accordingly. Do not set the directive to an unreleased version.

---

### Renovate automerge scope includes all non-major updates

- Issue: `renovate.json` sets `"automerge": true` globally for all non-major dependency updates. Minor and patch updates to `periph.io`, `prometheus/client_golang`, or `prokopparuzek/go-dht` can merge automatically without human review.
- Files: `renovate.json`
- Impact: Low for a single-binary exporter, but a bad patch in a transitive dep could silently break hardware communication.
- Fix approach: Restrict automerge to `patch` updates only (`"matchUpdateTypes": ["patch"]`) and require review for `minor` updates.

---

### No `RestartSec` or rate-limiting in systemd unit

- Issue: The unit has `Restart=on-failure` but no `RestartSec` or `StartLimitInterval`. A crash loop (e.g. from a nil panic) will respawn immediately and continuously.
- Files: `go-dht.service`
- Impact: Runaway restart loop consuming CPU and filling logs.
- Fix approach: Add `RestartSec=5` and `StartLimitIntervalSec=60` / `StartLimitBurst=5` to the `[Service]` section.

---

*Concerns audit: 2026-06-28*
