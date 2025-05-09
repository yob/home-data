package rules

import (
	"fmt"
	"sync"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
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

	//wg.Add(1)
	//go func() {
	//	acOffOnPriceSpikes(bus, logger, state)
	//	wg.Done()
	//}()

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

	wg.Add(1)
	go func() {
		effectivePrice(bus, logger, state)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		setPowerPricesLight(bus, logger, state)
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

//func acOffOnPriceSpikes(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
//	publish := bus.PublishChannel()
//	sub, _ := bus.Subscribe("every:minute")
//	defer sub.Close()
//
//	for _ = range sub.Ch {
//		logger.Debug("rules: executing acOffOnPriceSpikes")
//		effectiveCentsPerKwh, ok := state.ReadFloat64("effective_cents_per_kwh")
//		condOne := ok && effectiveCentsPerKwh > 150
//
//		logger.Debug(fmt.Sprintf("rules: evaluating acOffOnPriceSpikes - condOne: %t", condOne))
//
//		if condOne {
//			publish <- pubsub.PubsubEvent{
//				Topic: "daikin.kitchen.control",
//				Data:  pubsub.NewKeyValueEvent("power", "off"),
//			}
//
//			publish <- pubsub.PubsubEvent{
//				Topic: "daikin.study.control",
//				Data:  pubsub.NewKeyValueEvent("power", "off"),
//			}
//
//			publish <- pubsub.PubsubEvent{
//				Topic: "daikin.lounge.control",
//				Data:  pubsub.NewKeyValueEvent("power", "off"),
//			}
//
//			publish <- pubsub.PubsubEvent{
//				Topic: "email:send",
//				Data:  pubsub.NewEmailEvent("[home-data] Price spike! AC turned off", "I did a thing"),
//			}
//
//			publish <- pubsub.PubsubEvent{
//				Topic: "state:update",
//				Data:  pubsub.NewKeyValueEvent("acOffOnPriceSpikes_last_at", time.Now().UTC().Format(time.RFC3339)),
//			}
//		}
//	}
//}

func cheapPowerOn(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing cheapPowerOn")
		effectiveCentsPerKwh, ok := state.ReadFloat64("effective_cents_per_kwh")
		condOne := ok && effectiveCentsPerKwh < 18

		lowPricesOn, ok := state.Read("kasa.low-prices.on")
		condTwo := ok && lowPricesOn == "0"

		logger.Debug(fmt.Sprintf("rules: evaluating cheapPowerOn - condOne: %t condTwo: %t", condOne, condTwo))

		if condOne && condTwo {
			publish <- pubsub.PubsubEvent{
				Topic: "kasa.low-prices.control",
				Data:  pubsub.NewKeyValueEvent("power", "on"),
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
		effectiveCentsPerKwh, ok := state.ReadFloat64("effective_cents_per_kwh")
		condOne := ok && effectiveCentsPerKwh >= 18

		lowPricesOn, ok := state.Read("kasa.low-prices.on")
		condTwo := ok && lowPricesOn == "1"

		logger.Debug(fmt.Sprintf("rules: evaluating cheapPowerOff - condOne: %t condTwo: %t", condOne, condTwo))

		if condOne && condTwo {
			publish <- pubsub.PubsubEvent{
				Topic: "kasa.low-prices.control",
				Data:  pubsub.NewKeyValueEvent("power", "off"),
			}
			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("cheapPowerOff_last_at", time.Now().UTC().Format(time.RFC3339)),
			}
		}
	}
}

func effectivePrice(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing effectivePrice")

		reampedGeneralCentsPerKwh, ok := state.ReadFloat64("reamped.general.cents_per_kwh")
		condOne := ok

		gridDrawWatts, ok := state.ReadFloat64("fronius.inverter.grid_draw_watts")
		condTwo := ok

		condThree := gridDrawWatts <= 0

		logger.Debug(fmt.Sprintf("rules: evaluating effectivePrice - condOne: %t condTwo: %t condThree: %t", condOne, condTwo, condThree))

		effectivePriceSensor := entities.NewSensorGauge(bus, "effective_cents_per_kwh")

		// We're exporting to the grid, so we're generating more than we're using and electricity is free to use!
		if condOne && condTwo && condThree {
			effectivePriceSensor.Update(0)
		}

		// We're importing from the grid, so we're paying grid price
		if condOne && condTwo && !condThree {
			effectivePriceSensor.Update(reampedGeneralCentsPerKwh)
		}
	}
}

func setPowerPricesLight(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader) {
	publish := bus.PublishChannel()
	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		logger.Debug("rules: executing setPowerPricesLight")
		effectiveCentsPerKwh, ok := state.ReadFloat64("effective_cents_per_kwh")
		condOne := ok && effectiveCentsPerKwh < 20
		condTwo := ok && effectiveCentsPerKwh >= 20 && effectiveCentsPerKwh < 21
		condThree := ok && effectiveCentsPerKwh >= 21

		logger.Debug(fmt.Sprintf("rules: evaluating setPowerPricesLight - condOne: %t condTwo: %t condThree: %t", condOne, condTwo, condThree))

		if condOne { // green
			publish <- pubsub.PubsubEvent{
				Topic: "lifx.energylight.control",
				Data:  pubsub.NewKeyValueEvent("color:set", "26250,65535,39403,3500"),
			}
		} else if condTwo { // orange
			publish <- pubsub.PubsubEvent{
				Topic: "lifx.energylight.control",
				Data:  pubsub.NewKeyValueEvent("color:set", "4480,65535,39403,3500"),
			}
		} else if condThree { // red
			publish <- pubsub.PubsubEvent{
				Topic: "lifx.energylight.control",
				Data:  pubsub.NewKeyValueEvent("color:set", "1289,65535,39403,3500"),
			}
		}
	}
}
