package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/prokopparuzek/go-dht"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/wasilak/loggergo"

	"log/slog"
)

var k = koanf.New(".")

var version string

func recordMetrics(dhtInstance *dht.DHT, pin string, model string) {
	var err error

	temperatureMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensor_temperature",
		Help: "temperature from sensor",
	}, []string{"model", "pin"})

	humidityMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensor_humidity",
		Help: "humidity from sensor",
	}, []string{"model", "pin"})

	err = prometheus.Register(temperatureMetric)
	if err != nil {
		slog.Error(err.Error())
	}

	err = prometheus.Register(humidityMetric)
	if err != nil {
		slog.Error(err.Error())
	}

	go func() {
		for {
			humidity, temperature, err := dhtRun(dhtInstance, 10)
			if err != nil {
				slog.Error(err.Error())
			}

			temperatureMetric.With(prometheus.Labels{
				"model": model,
				"pin":   pin,
			}).Set(temperature)

			humidityMetric.With(prometheus.Labels{
				"model": model,
				"pin":   pin,
			}).Set(humidity)

			time.Sleep(10 * time.Second)
		}
	}()
}

func dhtSetup(pin string, model string) (*dht.DHT, error) {
	err := dht.HostInit()
	if err != nil {
		return nil, fmt.Errorf("HostInit error: %s", err)
	}

	dhtInstance, err := dht.NewDHT(fmt.Sprintf("GPIO%s", pin), dht.Celsius, model)
	if err != nil {
		return nil, fmt.Errorf("NewDHT error: %s", err)
	}

	return dhtInstance, nil
}

func dhtRun(dht *dht.DHT, retry int) (float64, float64, error) {
	humidity, temperature, err := dht.ReadRetry(retry)
	if err != nil {
		return 0, 0, fmt.Errorf("read error: %s", err)
	}

	slog.Debug(fmt.Sprintf("temperature: %v, humidity: %v", temperature, humidity))

	return humidity, temperature, nil
}

func main() {

	buildInfo, _ := debug.ReadBuildInfo()

	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("go-dht\nVersion %s\n", version)
		os.Exit(0)
	}

	k.Load(confmap.Provider(map[string]interface{}{
		"pin":       "27",
		"model":     "dht22",
		"debug":     true,
		"listen":    ":9877",
		"logLevel":  "info",
		"logFormat": "json",
	}, "."), nil)

	k.Load(env.Provider("GO_DHT_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "GO_DHT_")), "_", ".", -1)
	}), nil)

	pin := k.String("pin")
	model := k.String("model")

	loggergo.LoggerInit(k.String("logLevel"), k.String("logFormat"), slog.Int("pid", os.Getpid()), slog.String("go_version", buildInfo.GoVersion))

	if k.Bool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	dhtInstance, err := dhtSetup(pin, model)
	if err != nil {
		slog.Error(err.Error())
	}

	recordMetrics(dhtInstance, pin, model)

	http.Handle("/metrics", promhttp.Handler())

	err = http.ListenAndServe(k.String("listen"), nil)
	if err != nil {
		slog.Error(err.Error())
	}
}
