package rules

import (
	"fmt"
	"sync"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, configSection *conf.ConfigSection) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		kitchenHeatingOnColdMornings(bus, logger, state)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		acOffOnPriceSpikes(bus, logger, state)
		wg.Done()
	}()

	wg.Wait()
}

func kitchenHeatingOnColdMornings(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing kitchenHeatingOnColdMornings")
		// TODO only mon-fri
		condOne := time.Now().Hour() == 6

		lastAt, ok := state.ReadTime("kitchenHeatingOnColdMornings_last_at")
		condTwo := !ok || time.Since(lastAt) > 12*time.Hour

		jamesLastSeenAt, jamesOk := state.ReadTime("unifi.presence.last_seen.james")
		andreaLastSeenAt, andreaOk := state.ReadTime("unifi.presence.last_seen.andrea")
		condThree := (jamesOk && time.Since(jamesLastSeenAt) < 1*time.Hour) || (andreaOk && time.Since(andreaLastSeenAt) < 1*time.Hour)

		kitchenCelcius, ok := state.ReadFloat64("ruuvi.kitchen.temp_celcius")
		condFour := ok && kitchenCelcius <= 13

		logger.Debug(fmt.Sprintf("rules: evaluating kitchenHeatingOnColdMornings - condOne: %t, condTwo: %t, condThree: %t, condFour: %t", condOne, condTwo, condThree, condFour))

		if condOne && condTwo && condThree && condFour {
			publish <- pubsub.PubsubEvent{
				Topic: "daikin.kitchen.control",
				Data:  pubsub.NewKeyValueEvent("power", "on"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "email:send",
				Data:  pubsub.NewEmailEvent("[home-data] Cold morning - kitchen AC turned on", "I did a thing"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("kitchenHeatingOnColdMornings_last_at", time.Now().UTC().Format(time.RFC3339)),
			}
		}
	}
}

func acOffOnPriceSpikes(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing acOffOnPriceSpikes")
		amberGeneralCentsPerKwh, ok := state.ReadFloat64("amber.general.cents_per_kwh")
		condOne := ok && amberGeneralCentsPerKwh > 150

		logger.Debug(fmt.Sprintf("rules: evaluating acOffOnPriceSpikes - condOne: %t", condOne))

		if condOne {
			publish <- pubsub.PubsubEvent{
				Topic: "daikin.kitchen.control",
				Data:  pubsub.NewKeyValueEvent("power", "off"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "daikin.study.control",
				Data:  pubsub.NewKeyValueEvent("power", "off"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "daikin.lounge.control",
				Data:  pubsub.NewKeyValueEvent("power", "off"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "email:send",
				Data:  pubsub.NewEmailEvent("[home-data] Price spike! AC turned off", "I did a thing"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("acOffOnPriceSpikes_last_at", time.Now().UTC().Format(time.RFC3339)),
			}
		}
	}
}
