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

	wg.Add(1)
	go func() {
		reccomendOpenHouse(bus, logger, state)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cheapPowerOn(bus, logger, state)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cheapPowerOff(bus, logger, state)
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
		condFour := ok && kitchenCelcius <= 14

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

func reccomendOpenHouse(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing reccomendOpenHouse")
		outsideAbsHumidity, ok := state.ReadFloat64("ruuvi.outside.absolute_humidity_g_per_m3")
		condOne := ok && outsideAbsHumidity <= 7

		kitchenAbsHumidity, ok := state.ReadFloat64("ruuvi.kitchen.absolute_humidity_g_per_m3")
		condTwo := ok && kitchenAbsHumidity >= 9

		outsideTemp, ok := state.ReadFloat64("ruuvi.outside.temp_celcius")
		condThree := ok && outsideTemp >= 15

		condFour := ok && outsideTemp < 30

		lastAt, ok := state.ReadTime("reccomendOpenHouse_last_at")
		condFive := !ok || time.Since(lastAt) > 12*time.Hour

		logger.Debug(fmt.Sprintf("rules: evaluating reccomendOpenHouse - condOne: %t condTwo: %t condThree: %t condFour: %t condFive: %t", condOne, condTwo, condThree, condFour, condFive))

		if condOne && condTwo && condThree && condFour && condFive {

			publish <- pubsub.PubsubEvent{
				Topic: "email:send",
				Data:  pubsub.NewEmailEvent("[home-data] Reccommend opening the house", "Humidity inside is high, humidity outside is low, temp outside is mild. Get some fresh air flowing!"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("reccomendOpenHouse_last_at", time.Now().UTC().Format(time.RFC3339)),
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

func cheapPowerOn(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing cheapPowerOn")
		amberGeneralCentsPerKwh, ok := state.ReadFloat64("amber.general.cents_per_kwh")
		condOne := ok && amberGeneralCentsPerKwh < 18

		lowPricesOn, ok := state.Read("kasa.low-prices.on")
		condTwo := ok && lowPricesOn == "0"

		logger.Debug(fmt.Sprintf("rules: evaluating cheapPowerOn - condOne: %t condTwo: %t", condOne, condTwo))

		if condOne && condTwo {
			publish <- pubsub.PubsubEvent{
				Topic: "kasa.low-prices.control",
				Data:  pubsub.NewKeyValueEvent("power", "on"),
			}
			publish <- pubsub.PubsubEvent{
				Topic: "email:send",
				Data:  pubsub.NewEmailEvent("[home-data] Amber prices are cheap! Enabling low-power plugs", "I did a thing"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("cheapPowerOn_last_at", time.Now().UTC().Format(time.RFC3339)),
			}
		}
	}
}

func cheapPowerOff(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing cheapPowerOff")
		amberGeneralCentsPerKwh, ok := state.ReadFloat64("amber.general.cents_per_kwh")
		condOne := ok && amberGeneralCentsPerKwh >= 18

		lowPricesOn, ok := state.Read("kasa.low-prices.on")
		condTwo := ok && lowPricesOn == "1"

		logger.Debug(fmt.Sprintf("rules: evaluating cheapPowerOff - condOne: %t condTwo: %t", condOne, condTwo))

		if condOne && condTwo {
			publish <- pubsub.PubsubEvent{
				Topic: "kasa.low-prices.control",
				Data:  pubsub.NewKeyValueEvent("power", "off"),
			}
			publish <- pubsub.PubsubEvent{
				Topic: "email:send",
				Data:  pubsub.NewEmailEvent("[home-data] Amber prices aren't cheap any more! Turning off low-power plugs", "I did a thing"),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("cheapPowerOff_last_at", time.Now().UTC().Format(time.RFC3339)),
			}
		}
	}
}
