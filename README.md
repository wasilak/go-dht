# go-dht

[![CI](https://github.com/wasilak/go-dht/actions/workflows/main.yml/badge.svg)](https://github.com/wasilak/go-dht/actions/workflows/main.yml)
[![Maintainability](https://api.codeclimate.com/v1/badges/b1a1245e15f788148b03/maintainability)](https://codeclimate.com/github/wasilak/go-dht/maintainability)

Prometheus exporter for DHT11/DHT22 temperature and humidity sensors. Runs on Raspberry Pi, supports multiple sensors, and exposes structured metrics with per-sensor labels.

## Features

- Multiple sensors — any number of DHT11/DHT22 sensors on independent GPIO pins
- `sensor_up` gauge per sensor — failed reads are alertable, not silent
- Exponential backoff on read errors (10s → 5 min ceiling)
- Graceful shutdown on SIGTERM/SIGINT
- `/healthz` liveness endpoint
- Config file (YAML) + CLI flags + env vars, in that priority order
- Structured JSON logging with OpenTelemetry support
- Pre-built binaries for linux/amd64, linux/arm64, linux/armv5–7, darwin/amd64, darwin/arm64

## Metrics

| Metric | Labels | Description |
|---|---|---|
| `sensor_temperature` | `sensor` | Temperature in °C |
| `sensor_humidity` | `sensor` | Relative humidity in % |
| `sensor_up` | `sensor` | `1` if last read succeeded, `0` if it failed |

With `--extended-labels`, `model` and `pin` are added to all metrics.

```
sensor_temperature{sensor="garage"} 29.7
sensor_temperature{sensor="box"} 34.5
sensor_humidity{sensor="garage"} 55.1
sensor_humidity{sensor="box"} 48.3
sensor_up{sensor="garage"} 1
sensor_up{sensor="box"} 1
```

## Installation

Download the latest binary for your architecture from [Releases](https://github.com/wasilak/go-dht/releases):

```bash
# Raspberry Pi (64-bit OS)
wget https://github.com/wasilak/go-dht/releases/latest/download/go-dht-linux-arm64.zip
unzip go-dht-linux-arm64.zip
chmod +x go-dht
sudo mv go-dht /usr/local/bin/
```

## Configuration

Create `/etc/go-dht/go-dht.yaml`:

```yaml
sensors:
  - id: garage
    pin: 4
    model: dht22

  - id: box
    pin: 17
    model: dht22

listen: ":9877"
loglevel: info
logformat: json
extended-labels: false
```

**`id`** — label value used in Prometheus metrics. Pick something meaningful (`inside`, `outside`, `ambient`, `box`).  
**`pin`** — GPIO number (not physical pin number). GPIO4 = physical pin 7.  
**`model`** — `dht11` or `dht22`.

### Config file locations (searched in order)

1. `--config /path/to/config.yaml` (explicit)
2. `./go-dht.yaml`
3. `~/.go-dht.yaml`
4. `/etc/go-dht/go-dht.yaml`

### CLI flags

```
go-dht --help

Flags:
      --config string     config file path
      --extended-labels   expose model and pin as additional metric labels
      --listen string     address to listen on (default ":9877")
      --loglevel string   log level: debug, info, warn, error (default "info")
      --logformat string  log format: json, text (default "json")
```

### Environment variables

All flags are available as `GO_DHT_<FLAG>` env vars (hyphens → underscores):

```bash
GO_DHT_LISTEN=:9877
GO_DHT_LOGLEVEL=debug
GO_DHT_EXTENDED_LABELS=true
```

## Systemd

```bash
sudo mkdir -p /etc/go-dht
sudo cp go-dht.yaml /etc/go-dht/
sudo cp go-dht.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now go-dht
sudo systemctl status go-dht
```

The included `go-dht.service` runs as root (required for GPIO access) and includes:
- Automatic restart on failure (`RestartSec=5`, burst limit 5/60s)
- `NoNewPrivileges=true`, `PrivateTmp=true`

## Prometheus

```yaml
scrape_configs:
  - job_name: go-dht
    static_configs:
      - targets: ["localhost:9877"]
```

### Example alert

```yaml
- alert: SensorDown
  expr: sensor_up == 0
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "DHT sensor {{ $labels.sensor }} is not responding"
```

## Building from source

```bash
git clone https://github.com/wasilak/go-dht.git
cd go-dht
go build -o go-dht .
```

Cross-compile for Raspberry Pi:

```bash
GOOS=linux GOARCH=arm64 go build -o go-dht .
```

## Commands

```bash
go-dht               # start the exporter (requires config file)
go-dht version       # print version
go-dht --help        # show all flags
```
