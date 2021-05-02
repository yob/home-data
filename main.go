package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/yob/home-data/adapters/daikin"
	"github.com/yob/home-data/adapters/fronius"
	"github.com/yob/home-data/adapters/ruuvi"
	"github.com/yob/home-data/adapters/stackdriver"
	"github.com/yob/home-data/adapters/unifi"
	pub "github.com/yob/home-data/pubsub"
)

const (
	kitchen_ip      = "10.1.1.110"
	lounge_ip       = "10.1.1.111"
	study_ip        = "10.1.1.112"
	inverter_ip     = "10.1.1.69"
	unifi_ip        = "10.1.1.2"
	googleProjectID = "our-house-data"
)

var (
	state  = sync.Map{} // map[string]string{}
	pubsub = pub.NewPubsub()
)

type appHandler func(http.ResponseWriter, *http.Request) (int, error)

func main() {

	// all log messages printed via a single goroutine
	go func() {
		ch_log := pubsub.Subscribe("log:new")
		for event := range ch_log {
			fmt.Printf("%s: %s\n", event.Key, event.Value)
		}
	}()

	// update the shared state when attributes change
	go func() {
		ch_state_update := pubsub.Subscribe("state:update")
		for elem := range ch_state_update {
			stateUpdate(elem.Key, elem.Value)
		}
	}()

	// send data to stack driver every minute
	go func() {
		ch_every_minute := pubsub.Subscribe("every:minute")
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
		stackdriver.Process(googleProjectID, &state, stateMap, ch_every_minute)
	}()

	// trigger an event that anyone can listen to if they want to run code every minute
	go func() {
		everyMinuteEvent(pubsub.PublishChannel())
	}()

	// daikin plugin, one per unit
	go func() {
		daikin.Poll(pubsub.PublishChannel(), "kitchen", kitchen_ip, "")
	}()
	go func() {
		daikin.Poll(pubsub.PublishChannel(), "study", study_ip, os.Getenv("DAIKIN_STUDY_TOKEN"))
	}()
	go func() {
		daikin.Poll(pubsub.PublishChannel(), "lounge", lounge_ip, os.Getenv("DAIKIN_LOUNGE_TOKEN"))
	}()

	// fronius plugin, one per inverter
	go func() {
		fronius.Poll(pubsub.PublishChannel(), inverter_ip)
	}()

	// unifi plugin, one per network to detect presense of specific people
	go func() {
		config := unifi.Config{
			Address:   unifi_ip,
			UnifiUser: os.Getenv("UNIFI_USER"),
			UnifiPass: os.Getenv("UNIFI_PASS"),
			UnifiPort: os.Getenv("UNIFI_PORT"),
			UnifiSite: os.Getenv("UNIFI_SITE"),
			IpMap: map[string]string{
				"10.1.1.123": "james",
				"10.1.1.134": "andrea",
			},
		}
		unifi.Poll(pubsub.PublishChannel(), config)
	}()

	// webserver, as an alternative way to injest events
	go func() {
		startHttpServer()
	}()

	// loop forever, shuffling events between goroutines
	pubsub.Run()
}

func startHttpServer() {
	var addressMap = map[string]string{
		"cc:64:a6:ed:f6:aa": "study",
		"f2:b0:81:51:8a:e0": "bed1",
		"fb:dd:03:59:e8:26": "bed2",
		"ef:81:7d:23:3c:74": "lounge",
		"c2:69:9e:be:25:aa": "kitchen",
		"fd:54:a9:f0:a8:a5": "outside",
	}

	ruuviAdapter := ruuvi.NewRuuviAdapter(pubsub.PublishChannel(), addressMap)
	http.Handle("/ruuvi", appHandler(ruuviAdapter.HttpHandler))
	http.HandleFunc("/", http.NotFound)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if status, err := fn(w, r); err != nil {
		switch status {
		case http.StatusBadRequest:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case http.StatusNotFound:
			http.NotFound(w, r)
		case http.StatusInternalServerError:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			// Catch any other errors we haven't explicitly handled
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func stateUpdate(property string, value string) {
	existingValue, ok := state.Load(property)

	// if the property doesn't exist in the state yet, or it exists with a different value, then update it
	if !ok || existingValue != value {
		state.Store(property, value)
		fmt.Printf("set %s to %s\n", property, value)
	} else {
		//fmt.Printf("Skipped updating state for %s, no change in value\n", property)
	}
}

func everyMinuteEvent(publish chan pub.PubsubEvent) {
	lastBroadcast := time.Now()

	for {
		if time.Now().After(lastBroadcast.Add(time.Second * 60)) {
			publish <- pub.PubsubEvent{
				Topic: "every:minute",
				Data:  pub.KeyValueData{Key: "now", Value: time.Now().Format(time.RFC3339)},
			}
			lastBroadcast = time.Now()
		}
		time.Sleep(1 * time.Second)
	}
}
