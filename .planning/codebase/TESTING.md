# Testing Patterns

**Analysis Date:** 2026-06-28

## Test Framework

**Runner:** None — no test framework is configured.

**Config:** No `*_test.go` files exist. No `go test` targets defined.

**Run Commands:**
```bash
go build ./...     # Compile verification (primary quality gate)
go vet ./...       # Static analysis
```

## Existing Test Coverage

**Unit tests:** None.

**Integration tests:** None.

**E2E tests:** None.

The codebase has zero test files. The only automated quality signal is the GitHub Actions CI pipeline (`.github/`) which presumably runs `go build`.

## Build Verification

The primary correctness gate is compilation:

```bash
cd /Users/piotrek/git/go/go-dht
go build -o go-dht .
```

A compiled binary (`go-dht`) is committed to the repo (25.4 MB), which suggests the build is done locally and the artifact is checked in rather than built in CI from source.

**Vet:**
```bash
go vet ./...
```

No linter config (`.golangci.yml`, `staticcheck.conf`) is present.

## Manual Testing

The app requires physical DHT11/DHT22 hardware on a GPIO-capable host (Raspberry Pi). Full manual testing is hardware-dependent.

**Partial manual test (no hardware):**
```bash
# Test flag parsing and config loading without a real sensor
./go-dht --help
./go-dht version

# Test config file not found (silent) vs bad config (exit 1)
./go-dht --loglevel debug --listen :9877
# Expect: "no sensors configured" error because no config file present

# Test env var binding
GO_DHT_LOGLEVEL=debug GO_DHT_LOGFORMAT=text ./go-dht
```

**Test with a config file:**
```yaml
# go-dht.yaml
sensors:
  - id: test
    pin: "4"
    model: DHT22
```
```bash
./go-dht
# On non-GPIO host: expect HostInit error logged per sensor, metrics endpoint still starts
curl http://localhost:9877/metrics
```

## Gaps and Recommendations

**High priority:**

1. **No unit tests for pure logic** — `dhtRun`, `dhtSetup`, and `recordMetrics` are untestable as written because they call the hardware library directly. Wrap the DHT read behind an interface to enable mocking:
   ```go
   type SensorReader interface {
       ReadRetry(retries int) (humidity, temperature float64, err error)
   }
   ```
   Then `dhtRun` accepts a `SensorReader` and can be unit-tested without hardware.

2. **No test for config unmarshalling** — `viper.UnmarshalKey("sensors", &sensors)` with `sensorConfig` struct should be tested with valid and invalid YAML inputs.

3. **No test for env var → viper binding** — the `GO_DHT_*` env var prefix and hyphen-to-underscore replacement should be exercised in a test.

4. **Goroutine error handling untested** — `recordMetrics` silently swallows sensor read errors (logs only). A test should verify error logging occurs and the loop continues.

**Medium priority:**

5. **No linter** — add `golangci-lint` with at minimum `errcheck`, `govet`, `staticcheck`. The current `slog.Debug(fmt.Sprintf(...))` pattern would be caught by `sloglint`.

6. **No CI test step visible** — confirm `.github/workflows/` runs `go test ./...` (currently zero tests so it's a no-op, but the step should exist for when tests are added).

**Low priority:**

7. **Binary committed to repo** — the `go-dht` binary (25.4 MB) is checked in. This is unusual and inflates the repo. Consider adding it to `.gitignore` and building in CI/releasing via GitHub Releases instead.

---

*Testing analysis: 2026-06-28*
