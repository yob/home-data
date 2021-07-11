package daikin

import (
	"fmt"
	"sync"
	"time"

	daikinClient "github.com/buxtronix/go-daikin"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/pubsub"
)

type Config struct {
	Address string
	Name    string
	Token   string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, config Config) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		broadcastState(bus, logger, config)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		changeState(bus, logger, config)
		wg.Done()
	}()

	wg.Wait()
}

func broadcastState(bus *pubsub.Pubsub, logger *logging.Logger, config Config) {
	publish := bus.PublishChannel()
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.Address, config.Token))
	if err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.Name, err))
		return
	}

	dev := d.Devices[config.Address]
	if err := dev.GetControlInfo(); err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.Name, err))
		return
	}

	for {
		if err := dev.GetSensorInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.Name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.temp_inside_celcius", config.Name), dev.SensorInfo.HomeTemperature.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.temp_outside_celcius", config.Name), dev.SensorInfo.OutsideTemperature.String()),
		}

		if err := dev.GetControlInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.Name, err))
			continue
		}

		var powerInt = 0 // 0 == Off, 1 == On
		if dev.ControlInfo.Power.String() == "On" {
			powerInt = 1
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.power", config.Name), fmt.Sprintf("%d", powerInt)),
		}

		if err := dev.GetWeekPower(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.Name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.watt_hours_today", config.Name), dev.WeekPower.TodayWattHours.String()),
		}

		time.Sleep(20 * time.Second)
	}
}

func changeState(bus *pubsub.Pubsub, logger *logging.Logger, config Config) {
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.Address, config.Token))
	if err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.Name, err))
		return
	}

	dev := d.Devices[config.Address]
	if err := dev.GetControlInfo(); err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.Name, err))
		return
	}

	subControl, _ := bus.Subscribe(fmt.Sprintf("daikin.%s.control", config.Name))
	defer subControl.Close()

	for event := range subControl.Ch {
		if event.Key == "power" && event.Value == "off" {
			if err := dev.GetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): %v", config.Name, err))
				continue
			}

			dev.ControlInfo.Power = daikinClient.PowerOff
			if err := dev.SetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): error setting control: %v", config.Name, err))
				continue
			}
		} else if event.Key == "power" && event.Value == "on" {
			if err := dev.GetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): %v", config.Name, err))
				continue
			}

			dev.ControlInfo.Power = daikinClient.PowerOn
			if err := dev.SetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): error setting control: %v", config.Name, err))
				continue
			}
		} else {
			logger.Error(fmt.Sprintf("daikin (%s): unrecognised event: %v", config.Name, event))
		}
	}
}
