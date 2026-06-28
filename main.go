package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/prokopparuzek/go-dht"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wasilak/loggergo"
	loggergoLib "github.com/wasilak/loggergo/lib"
	loggergoTypes "github.com/wasilak/loggergo/lib/types"

	"log/slog"
)

var version string

type sensorConfig struct {
	ID    string
	Pin   string
	Model string
}

func parseSensors(cfg string) []sensorConfig {
	var sensors []sensorConfig
	for _, s := range strings.Split(cfg, ",") {
		parts := strings.SplitN(strings.TrimSpace(s), ":", 3)
		if len(parts) != 3 {
			slog.Error("invalid sensor config, expected id:pin:model", slog.String("sensor", s))
			continue
		}
		sensors = append(sensors, sensorConfig{ID: parts[0], Pin: parts[1], Model: parts[2]})
	}
	return sensors
}

func recordMetrics(dhtInstance *dht.DHT, sensor sensorConfig, tempGauge, humGauge *prometheus.GaugeVec, extendedLabels bool) {
	go func() {
		for {
			humidity, temperature, err := dhtRun(dhtInstance, 10)
			if err != nil {
				slog.Error(err.Error(), slog.String("sensor", sensor.ID))
			}

			labels := prometheus.Labels{"sensor": sensor.ID}
			if extendedLabels {
				labels["model"] = sensor.Model
				labels["pin"] = sensor.Pin
			}

			tempGauge.With(labels).Set(temperature)
			humGauge.With(labels).Set(humidity)

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

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-dht",
		Short: "DHT11/DHT22 Prometheus exporter",
		RunE:  runServer,
	}

	cmd.Flags().String("sensors", "default:27:dht22", "comma-separated list of sensors: id:pin:model,...")
	cmd.Flags().Bool("extended-labels", false, "expose model and pin as additional metric labels")
	cmd.Flags().String("listen", ":9877", "address to listen on")
	cmd.Flags().String("loglevel", "info", "log level (debug, info, warn, error)")
	cmd.Flags().String("logformat", "json", "log format (json, text)")

	viper.BindPFlags(cmd.Flags())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("go-dht\nVersion %s\n", version)
		},
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	buildInfo, _ := debug.ReadBuildInfo()

	loggerConfig := loggergoTypes.Config{
		Level:  loggergoLib.LogLevelFromString(viper.GetString("loglevel")),
		Format: loggergoLib.LogFormatFromString(viper.GetString("logformat")),
	}

	loggergo.Init(ctx, loggerConfig, slog.Int("pid", os.Getpid()), slog.String("go_version", buildInfo.GoVersion))

	extendedLabels := viper.GetBool("extended-labels")

	labelNames := []string{"sensor"}
	if extendedLabels {
		labelNames = append(labelNames, "model", "pin")
	}

	tempGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensor_temperature",
		Help: "temperature from sensor",
	}, labelNames)

	humGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensor_humidity",
		Help: "humidity from sensor",
	}, labelNames)

	prometheus.MustRegister(tempGauge, humGauge)

	sensors := parseSensors(viper.GetString("sensors"))

	for _, sensor := range sensors {
		dhtInstance, err := dhtSetup(sensor.Pin, sensor.Model)
		if err != nil {
			slog.Error(err.Error(), slog.String("sensor", sensor.ID))
			continue
		}
		recordMetrics(dhtInstance, sensor, tempGauge, humGauge, extendedLabels)
	}

	http.Handle("/metrics", promhttp.Handler())

	if err := http.ListenAndServe(viper.GetString("listen"), nil); err != nil {
		return err
	}

	return nil
}

func main() {
	viper.SetEnvPrefix("GO_DHT")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	root := newRootCmd()
	root.AddCommand(newVersionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
