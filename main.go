package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yob/home-data/core/config"
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
	"github.com/yob/home-data/adapters/unifi"
	pub "github.com/yob/home-data/pubsub"
)

func main() {
	pubsub := pub.NewPubsub()
	state := memorystate.New()

	configFile, err := config.NewConfigFromFile("config.toml")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading config file: %v", err))
	}

	coreConfig, err := configFile.Section("core")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading core section from config file: %v", err))
	}

	amberConfig, err := configFile.Section("amber")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading amber section from config file: %v", err))
	}

	froniusConfig, err := configFile.Section("fronius")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading fronius section from config file: %v", err))
	}

	datadogConfig, err := configFile.Section("datadog")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading datadog section from config file: %v", err))
	}

	daikinLoungeConfig, err := configFile.Section("ac-lounge")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading ac-lounge section from config file: %v", err))
	}

	daikinStudyConfig, err := configFile.Section("ac-study")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading ac-study section from config file: %v", err))
	}

	daikinKitchenConfig, err := configFile.Section("ac-kitchen")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading ac-kitchen section from config file: %v", err))
	}

	ruuviConfig, err := configFile.Section("ruuvi")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading ruuvi section from config file: %v", err))
	}

	unifiConfig, err := configFile.Section("unifi")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading unifi section from config file: %v", err))
	}

	// webserver, as an alternative way to injest events
	go func() {
		http.Init(pubsub, coreConfig)
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

	// send data to datadog every minute
	go func() {
		logger := logging.NewLogger(pubsub)
		datadog.Init(pubsub, logger, state.ReadOnly(), datadogConfig)
	}()

	// daikin plugin, one per unit
	go func() {
		logger := logging.NewLogger(pubsub)
		daikin.Init(pubsub, logger, state.ReadOnly(), daikinKitchenConfig)
	}()
	go func() {
		logger := logging.NewLogger(pubsub)
		daikin.Init(pubsub, logger, state.ReadOnly(), daikinStudyConfig)
	}()
	go func() {
		logger := logging.NewLogger(pubsub)
		daikin.Init(pubsub, logger, state.ReadOnly(), daikinLoungeConfig)
	}()

	// amber plugin
	go func() {
		logger := logging.NewLogger(pubsub)
		amber.Init(pubsub, logger, state.ReadOnly(), amberConfig)
	}()

	// fronius plugin, one per inverter
	go func() {
		logger := logging.NewLogger(pubsub)
		fronius.Init(pubsub, logger, state.ReadOnly(), froniusConfig)
	}()

	// ruuvi plugin
	go func() {
		logger := logging.NewLogger(pubsub)
		ruuvi.Init(pubsub, logger, state.ReadOnly(), ruuviConfig)
	}()

	// unifi plugin, one per network to detect presense of specific people
	go func() {
		logger := logging.NewLogger(pubsub)
		unifi.Init(pubsub, logger, state.ReadOnly(), unifiConfig)
	}()

	// loop forever, shuffling events between goroutines
	pubsub.Run()
}
