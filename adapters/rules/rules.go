package rules

import (
	"sync"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, configSection *conf.ConfigSection) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		kitchenHeatingOnColdMornings(bus, logger, state)
		wg.Done()
	}()

	wg.Wait()
}

func kitchenHeatingOnColdMornings(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("executing rule kitchenHeatingOnColdMornings")
		// TODO only mon-fri
		if time.Now().Hour() == 6 {
			lastAt, ok := state.ReadTime("kitchenHeatingOnColdMornings_last_at")
			if !ok || time.Since(lastAt) > 12*time.Hour {
				jamesLastSeenAt, jamesOk := state.ReadTime("unifi.presence.last_seen.james")
				andreaLastSeenAt, andreaOk := state.ReadTime("unifi.presence.last_seen.andrea")
				if (jamesOk && time.Since(jamesLastSeenAt) < 1*time.Hour) || (andreaOk && time.Since(andreaLastSeenAt) < 1*time.Hour) {
					if kitchenCelcius, ok := state.ReadFloat64("ruuvi.kitchen.temp_celcius"); ok {
						if kitchenCelcius <= 13 {
							publish <- pubsub.PubsubEvent{
								Topic: "daikin.kitchen.control",
								Data:  pubsub.NewKeyValueEvent("power", "on"),
							}

							publish <- pubsub.PubsubEvent{
								Topic: "state:update",
								Data:  pubsub.NewKeyValueEvent("kitchenHeatingOnColdMornings_last_at", time.Now().UTC().Format(time.RFC3339)),
							}
						}
					}
				}
			}
		}
	}
}
