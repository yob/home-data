package main

import (
	"os"
	"time"

	"github.com/yob/home-data/core/http"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/core/statebus"
	"github.com/yob/home-data/core/timers"

	"github.com/yob/home-data/adapters/amber"
	"github.com/yob/home-data/adapters/daikin"
	"github.com/yob/home-data/adapters/datadog"
	"github.com/yob/home-data/adapters/fronius"
	"github.com/yob/home-data/adapters/ruuvi"
	"github.com/yob/home-data/adapters/stackdriver"
	"github.com/yob/home-data/adapters/unifi"
	pub "github.com/yob/home-data/pubsub"
)

func main() {
	pubsub := pub.NewPubsub()
	state := memorystate.New()

	// webserver, as an alternative way to injest events
	go func() {
		http.Init(pubsub, 8080)
	}()

	// all log messages printed via a single goroutine
	go func() {
		logging.Init(pubsub)
	}()

	// trigger events at reliable intervals for anyone can listen to if they want to run code
	// regularly
	go func() {
		timers.Init(pubsub)
	}()

	// update the shared state when attributes change
	go func() {
		logger := logging.NewLogger(pubsub)
		statebus.Init(pubsub, logger, state)
	}()

	// Give the core functions time to setup before we start registering adapters.
	// TODO: replace this with proper signaling when all core functions are ready.
	time.Sleep(2 * time.Second)

	// send data to stack driver every minute
	go func() {
		stateMap := map[string]string{
			"daikin.kitchen.temp_inside_celcius":  "daikin.kitchen.inside_temp",
			"daikin.kitchen.temp_outside_celcius": "daikin.kitchen.outside_temp",
			"daikin.kitchen.power":                "daikin.kitchen.power",
			"daikin.kitchen.watt_hours_today":     "daikin.kitchen.power_watt_hours",
			"daikin.lounge.temp_inside_celcius":   "daikin.lounge.inside_temp",
			"daikin.lounge.temp_outside_celcius":  "daikin.lounge.outside_temp",
			"daikin.lounge.power":                 "daikin.lounge.power",
			"daikin.lounge.watt_hours_today":      "daikin.lounge.power_watt_hours",
			"daikin.study.temp_inside_celcius":    "daikin.study.inside_temp",
			"daikin.study.temp_outside_celcius":   "daikin.study.outside_temp",
			"daikin.study.power":                  "daikin.study.power",
			"daikin.study.watt_hours_today":       "daikin.study.power_watt_hours",

			"fronius.inverter.grid_draw_watts":       "grid_draw_watts",
			"fronius.inverter.power_watts":           "power_watts",
			"fronius.inverter.generation_watts":      "generation_watts",
			"fronius.inverter.energy_day_watt_hours": "energy_day_watt_hours",
			"fronius.inverter.grid_voltage":          "grid_voltage",

			"ruuvi.study.temp_celcius": "ruuvi.study.temp",
			"ruuvi.study.humidity":     "ruuvi.study.humidity",
			"ruuvi.study.pressure":     "ruuvi.study.pressure",

			"ruuvi.bed1.temp_celcius": "ruuvi.bed1.temp",
			"ruuvi.bed1.humidity":     "ruuvi.bed1.humidity",
			"ruuvi.bed1.pressure":     "ruuvi.bed1.pressure",

			"ruuvi.bed2.temp_celcius": "ruuvi.bed2.temp",
			"ruuvi.bed2.humidity":     "ruuvi.bed2.humidity",
			"ruuvi.bed2.pressure":     "ruuvi.bed2.pressure",

			"ruuvi.lounge.temp_celcius": "ruuvi.lounge.temp",
			"ruuvi.lounge.humidity":     "ruuvi.lounge.humidity",
			"ruuvi.lounge.pressure":     "ruuvi.lounge.pressure",

			"ruuvi.kitchen.temp_celcius": "ruuvi.kitchen.temp",
			"ruuvi.kitchen.humidity":     "ruuvi.kitchen.humidity",
			"ruuvi.kitchen.pressure":     "ruuvi.kitchen.pressure",

			"ruuvi.outside.temp_celcius": "ruuvi.outside.temp",
			"ruuvi.outside.humidity":     "ruuvi.outside.humidity",
			"ruuvi.outside.pressure":     "ruuvi.outside.pressure",
		}
		googleProjectID := "our-house-data"
		logger := logging.NewLogger(pubsub)
		stackdriver.Init(pubsub, logger, googleProjectID, state.ReadOnly(), stateMap)
	}()

	// send data to datadog every minute
	go func() {
		interestingKeys := []string{
			"daikin.kitchen.temp_inside_celcius",
			"daikin.kitchen.temp_outside_celcius",
			"daikin.kitchen.power",
			"daikin.kitchen.watt_hours_today",
			"daikin.lounge.temp_inside_celcius",
			"daikin.lounge.temp_outside_celcius",
			"daikin.lounge.power",
			"daikin.lounge.watt_hours_today",
			"daikin.study.temp_inside_celcius",
			"daikin.study.temp_outside_celcius",
			"daikin.study.power",
			"daikin.study.watt_hours_today",

			"fronius.inverter.grid_draw_watts",
			"fronius.inverter.power_watts",
			"fronius.inverter.generation_watts",
			"fronius.inverter.energy_day_watt_hours",
			"fronius.inverter.grid_voltage",

			"ruuvi.study.temp_celcius",
			"ruuvi.study.humidity",
			"ruuvi.study.pressure",
			"ruuvi.study.dewpoint_celcius",
			"ruuvi.study.absolute_humidity_g_per_m3",

			"ruuvi.bed1.temp_celcius",
			"ruuvi.bed1.humidity",
			"ruuvi.bed1.pressure",
			"ruuvi.bed1.dewpoint_celcius",
			"ruuvi.bed1.absolute_humidity_g_per_m3",

			"ruuvi.bed2.temp_celcius",
			"ruuvi.bed2.humidity",
			"ruuvi.bed2.pressure",
			"ruuvi.bed2.dewpoint_celcius",
			"ruuvi.bed2.absolute_humidity_g_per_m3",

			"ruuvi.lounge.temp_celcius",
			"ruuvi.lounge.humidity",
			"ruuvi.lounge.pressure",
			"ruuvi.lounge.dewpoint_celcius",
			"ruuvi.lounge.absolute_humidity_g_per_m3",

			"ruuvi.kitchen.temp_celcius",
			"ruuvi.kitchen.humidity",
			"ruuvi.kitchen.pressure",
			"ruuvi.kitchen.dewpoint_celcius",
			"ruuvi.kitchen.absolute_humidity_g_per_m3",

			"ruuvi.outside.temp_celcius",
			"ruuvi.outside.humidity",
			"ruuvi.outside.pressure",
			"ruuvi.outside.dewpoint_celcius",
			"ruuvi.outside.absolute_humidity_g_per_m3",

			"amber.general.cents_per_kwh",
			"amber.general.spot_cents_per_kwh",
			"amber.general.renewables",
			"amber.feedin.cents_per_kwh",
		}
		logger := logging.NewLogger(pubsub)
		datadog.Init(pubsub, logger, state.ReadOnly(), interestingKeys)
	}()

	// daikin plugin, one per unit
	go func() {
		config := daikin.Config{
			Address: "10.1.1.110",
			Name:    "kitchen",
		}
		logger := logging.NewLogger(pubsub)
		daikin.Init(pubsub, logger, config)
	}()
	go func() {
		config := daikin.Config{
			Address: "10.1.1.112",
			Name:    "study",
			Token:   os.Getenv("DAIKIN_STUDY_TOKEN"),
		}
		logger := logging.NewLogger(pubsub)
		daikin.Init(pubsub, logger, config)
	}()
	go func() {
		config := daikin.Config{
			Address: "10.1.1.111",
			Name:    "lounge",
			Token:   os.Getenv("DAIKIN_LOUNGE_TOKEN"),
		}
		logger := logging.NewLogger(pubsub)
		daikin.Init(pubsub, logger, config)
	}()

	// amber plugin
	go func() {
		logger := logging.NewLogger(pubsub)
		amber.Init(pubsub, logger, os.Getenv("AMBER_API_KEY"))
	}()

	// fronius plugin, one per inverter
	go func() {
		logger := logging.NewLogger(pubsub)
		fronius.Init(pubsub, logger, "10.1.1.69")
	}()

	// ruuvi plugin
	go func() {
		var addressmap = map[string]string{
			"cc:64:a6:ed:f6:aa": "study",
			"f2:b0:81:51:8a:e0": "bed1",
			"fb:dd:03:59:e8:26": "bed2",
			"ef:81:7d:23:3c:74": "lounge",
			"c2:69:9e:be:25:aa": "kitchen",
			"fd:54:a9:f0:a8:a5": "outside",
		}

		logger := logging.NewLogger(pubsub)
		ruuvi.Init(pubsub, logger, addressmap)
	}()

	// unifi plugin, one per network to detect presense of specific people
	go func() {
		config := unifi.Config{
			Address:   "10.1.1.2",
			UnifiUser: os.Getenv("UNIFI_USER"),
			UnifiPass: os.Getenv("UNIFI_PASS"),
			UnifiPort: os.Getenv("UNIFI_PORT"),
			UnifiSite: os.Getenv("UNIFI_SITE"),
			IpMap: map[string]string{
				"10.1.1.123": "james",
				"10.1.1.134": "andrea",
			},
		}
		logger := logging.NewLogger(pubsub)
		unifi.Init(pubsub, logger, config)
	}()

	// loop forever, shuffling events between goroutines
	pubsub.Run()
}
