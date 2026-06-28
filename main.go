package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
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
	ID    string `mapstructure:"id"`
	Pin   string `mapstructure:"pin"`
	Model string `mapstructure:"model"`
}

type gauges struct {
	temperature *prometheus.GaugeVec
	humidity    *prometheus.GaugeVec
	up          *prometheus.GaugeVec
}

func sensorLabels(sensor sensorConfig, extended bool) prometheus.Labels {
	labels := prometheus.Labels{"sensor": sensor.ID}
	if extended {
		labels["model"] = sensor.Model
		labels["pin"] = sensor.Pin
	}
	return labels
}

func recordMetrics(dhtInstance *dht.DHT, sensor sensorConfig, g gauges, extendedLabels bool) {
	go func() {
		backoff := 10 * time.Second
		const maxBackoff = 5 * time.Minute

		for {
			humidity, temperature, err := dhtRun(dhtInstance, 10)
			if err != nil {
				slog.Error(err.Error(), slog.String("sensor", sensor.ID))
				g.up.With(sensorLabels(sensor, extendedLabels)).Set(0)
				time.Sleep(backoff)
				backoff = min(backoff*2, maxBackoff)
				continue
			}

			backoff = 10 * time.Second
			labels := sensorLabels(sensor, extendedLabels)
			g.up.With(labels).Set(1)
			g.temperature.With(labels).Set(temperature)
			g.humidity.With(labels).Set(humidity)

			time.Sleep(backoff)
		}
	}()
}

func dhtSetup(pin string, model string) (*dht.DHT, error) {
	dhtInstance, err := dht.NewDHT(fmt.Sprintf("GPIO%s", pin), dht.Celsius, model)
	if err != nil {
		return nil, fmt.Errorf("NewDHT error: %w", err)
	}
	return dhtInstance, nil
}

func dhtRun(d *dht.DHT, retry int) (float64, float64, error) {
	humidity, temperature, err := d.ReadRetry(retry)
	if err != nil {
		return 0, 0, fmt.Errorf("read error: %w", err)
	}
	slog.Debug("sensor read", slog.Float64("temperature", temperature), slog.Float64("humidity", humidity))
	return humidity, temperature, nil
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-dht",
		Short: "DHT11/DHT22 Prometheus exporter",
		RunE:  runServer,
	}

	cmd.PersistentFlags().String("config", "", "config file (default: ./go-dht.yaml, ~/.go-dht.yaml, /etc/go-dht/go-dht.yaml)")
	cmd.Flags().Bool("extended-labels", false, "expose model and pin as additional metric labels")
	cmd.Flags().String("listen", ":9877", "address to listen on")
	cmd.Flags().String("loglevel", "info", "log level (debug, info, warn, error)")
	cmd.Flags().String("logformat", "json", "log format (json, text)")

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		panic(fmt.Sprintf("failed to bind flags: %v", err))
	}

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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	buildInfo, _ := debug.ReadBuildInfo()
	goVersion := "unknown"
	if buildInfo != nil {
		goVersion = buildInfo.GoVersion
	}

	loggerConfig := loggergoTypes.Config{
		Level:  loggergoLib.LogLevelFromString(viper.GetString("loglevel")),
		Format: loggergoLib.LogFormatFromString(viper.GetString("logformat")),
	}
	loggergo.Init(ctx, loggerConfig, slog.Int("pid", os.Getpid()), slog.String("go_version", goVersion))

	if err := dht.HostInit(); err != nil {
		return fmt.Errorf("HostInit error: %w", err)
	}

	var sensors []sensorConfig
	if err := viper.UnmarshalKey("sensors", &sensors); err != nil {
		return fmt.Errorf("invalid sensors config: %w", err)
	}
	if len(sensors) == 0 {
		return fmt.Errorf("no sensors configured")
	}

	extendedLabels := viper.GetBool("extended-labels")
	labelNames := []string{"sensor"}
	if extendedLabels {
		labelNames = append(labelNames, "model", "pin")
	}

	g := gauges{
		temperature: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "sensor_temperature", Help: "temperature from sensor"}, labelNames),
		humidity:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "sensor_humidity", Help: "humidity from sensor"}, labelNames),
		up:          prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "sensor_up", Help: "1 if last sensor read succeeded, 0 otherwise"}, labelNames),
	}
	prometheus.MustRegister(g.temperature, g.humidity, g.up)

	started := 0
	for _, sensor := range sensors {
		dhtInstance, err := dhtSetup(sensor.Pin, sensor.Model)
		if err != nil {
			slog.Error(err.Error(), slog.String("sensor", sensor.ID))
			continue
		}
		recordMetrics(dhtInstance, sensor, g, extendedLabels)
		started++
	}
	if started == 0 {
		return fmt.Errorf("all sensors failed to initialize")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: viper.GetString("listen"), Handler: mux}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown error", slog.String("err", err.Error()))
		}
	}()

	slog.Info("starting", slog.String("addr", viper.GetString("listen")))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func initConfig(cmd *cobra.Command) {
	if cfgFile, _ := cmd.Flags().GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("go-dht")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME")
		viper.AddConfigPath("/etc/go-dht")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintln(os.Stderr, "config error:", err)
			os.Exit(1)
		}
	}
}

func main() {
	viper.SetEnvPrefix("GO_DHT")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	root := newRootCmd()
	root.AddCommand(newVersionCmd())

	cobra.OnInitialize(func() { initConfig(root) })

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
