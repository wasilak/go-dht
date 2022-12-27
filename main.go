package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/prokopparuzek/go-dht"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var k = koanf.New(".")

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
		log.Fatal(err)
	}

	err = prometheus.Register(humidityMetric)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			humidity, temperature, err := dhtRun(dhtInstance, 10)
			if err != nil {
				log.Error(err)
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

	log.Debugf("temperature: %v, humidity: %v", temperature, humidity)

	return humidity, temperature, nil
}

func main() {

	k.Load(confmap.Provider(map[string]interface{}{
		"pin":    "27",
		"model":  "dht22",
		"debug":  true,
		"listen": ":9877",
	}, "."), nil)

	k.Load(env.Provider("GO_DHT_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "GO_DHT_")), "_", ".", -1)
	}), nil)

	pin := k.String("pin")
	model := k.String("model")

	if k.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	dhtInstance, err := dhtSetup(pin, model)
	if err != nil {
		log.Fatal(err)
	}

	recordMetrics(dhtInstance, pin, model)

	http.Handle("/metrics", promhttp.Handler())

	err = http.ListenAndServe(k.String("listen"), nil)
	if err != nil {
		log.Fatal(err)
	}
}
