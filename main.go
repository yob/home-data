package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yob/home-data/core/config"
	_ "github.com/yob/home-data/core/crdbstate"
	"github.com/yob/home-data/core/email"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/http"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/core/statebus"
	"github.com/yob/home-data/core/timers"

	"github.com/yob/home-data/adapters/amber"
	"github.com/yob/home-data/adapters/daikin"
	"github.com/yob/home-data/adapters/datadog"
	"github.com/yob/home-data/adapters/fronius"
	"github.com/yob/home-data/adapters/rules"
	"github.com/yob/home-data/adapters/ruuvi"
	"github.com/yob/home-data/adapters/unifi"
	pub "github.com/yob/home-data/pubsub"
)

func main() {
	adapterFuncs := map[string]func(*pub.Pubsub, *logging.Logger, homestate.StateReader, *config.ConfigSection){
		"amber":   amber.Init,
		"daikin":  daikin.Init,
		"datadog": datadog.Init,
		"fronius": fronius.Init,
		"rules":   rules.Init,
		"ruuvi":   ruuvi.Init,
		"unifi":   unifi.Init,
	}
	pubsub := pub.NewPubsub()
	state := memorystate.New()

	configPath, err := config.FindConfigPath()
	if err != nil {
		log.Fatal(err)
	}

	configFile, err := config.NewConfigFromFile(configPath)
	if err != nil {
		log.Fatal(fmt.Sprintf("error reading config file: %v", err))
	}

	coreConfig, err := configFile.Section("core")
	if err != nil {
		log.Fatal(fmt.Sprintf("Error reading core section from config file: %v", err))
	}

	// all log messages printed via a single goroutine
	go func() {
		logging.Init(pubsub)
	}()
	err = pubsub.WaitUntilSubscriber("log:new", 5)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error initializing logger: %v", err))
	}

	// update the shared state when attributes change
	go func() {
		logger := logging.NewLogger(pubsub)
		statebus.Init(pubsub, logger, state)
	}()
	err = pubsub.WaitUntilSubscriber("state:update", 5)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error initializing statebus: %v", err))
	}

	// send emails
	go func() {
		logger := logging.NewLogger(pubsub)
		email.Init(pubsub, logger, coreConfig)
	}()
	// TODO is it a fatal error if email is misconfigured?
	// TODO should we block until the email subscriber is listening?

	// webserver, as an alternative way to injest events
	go func() {
		http.Init(pubsub, coreConfig)
	}()
	err = pubsub.WaitUntilSubscriber("http:register-path", 5)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error initializing http: %v", err))
	}

	// trigger events at reliable intervals so anyone can listen to if they want to run code
	// regularly
	go func() {
		timers.Init(pubsub)
	}()

	// TEMP: debugging
	go func() {
		for {
			length, capacity := pubsub.PublishChanStats()
			fmt.Printf("publish - length: %d capacity: %d\n", length, capacity)
			time.Sleep(10 * time.Second)
		}
	}()

	// Now that core is all ready, load any adapters listed in the config file.
	for _, adapterSection := range configFile.AdapterSections() {
		adapterName, _ := adapterSection.GetString("adapter")
		localSection := adapterSection
		logger := logging.NewLogger(pubsub)
		if initFunc, ok := adapterFuncs[adapterName]; ok {
			go func() {
				initFunc(pubsub, logger, state.ReadOnly(), localSection)
			}()
		} else {
			logger.Fatal(fmt.Sprintf("adapter '%s' not recognised", adapterName))
		}
	}

	// loop forever, shuffling events between goroutines
	pubsub.Run()
}
