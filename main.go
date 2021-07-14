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
	adapterFuncs := map[string]func(*pub.Pubsub, *logging.Logger, memorystate.StateReader, *config.ConfigSection){
		"amber":   amber.Init,
		"daikin":  daikin.Init,
		"datadog": datadog.Init,
		"fronius": fronius.Init,
		"ruuvi":   ruuvi.Init,
		"unifi":   unifi.Init,
	}
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

	for _, adapterSection := range configFile.AdapterSections() {
		adapterName, _ := adapterSection.GetString("adapter")
		localSection := adapterSection
		go func() {
			logger := logging.NewLogger(pubsub)
			initFunc := adapterFuncs[adapterName]
			initFunc(pubsub, logger, state.ReadOnly(), localSection)
		}()
	}

	// loop forever, shuffling events between goroutines
	pubsub.Run()
}
