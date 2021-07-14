package daikin

import (
	"fmt"
	"sync"
	"time"

	daikinClient "github.com/buxtronix/go-daikin"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/pubsub"
)

type configData struct {
	address string
	name    string
	token   string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, configSection *conf.ConfigSection) {
	var wg sync.WaitGroup

	config, err := newConfigFromSection(configSection)
	if err != nil {
		logger.Fatal(fmt.Sprintf("daikin: %v", err))
		return
	}

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

func broadcastState(bus *pubsub.Pubsub, logger *logging.Logger, config configData) {
	publish := bus.PublishChannel()
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.address, config.token))
	if err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.name, err))
		return
	}

	dev := d.Devices[config.address]
	if err := dev.GetControlInfo(); err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.name, err))
		return
	}

	for {
		if err := dev.GetSensorInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.temp_inside_celcius", config.name), dev.SensorInfo.HomeTemperature.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.temp_outside_celcius", config.name), dev.SensorInfo.OutsideTemperature.String()),
		}

		if err := dev.GetControlInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		var powerInt = 0 // 0 == Off, 1 == On
		if dev.ControlInfo.Power.String() == "On" {
			powerInt = 1
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.power", config.name), fmt.Sprintf("%d", powerInt)),
		}

		if err := dev.GetWeekPower(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("daikin.%s.watt_hours_today", config.name), dev.WeekPower.TodayWattHours.String()),
		}

		time.Sleep(20 * time.Second)
	}
}

func changeState(bus *pubsub.Pubsub, logger *logging.Logger, config configData) {
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.address, config.token))
	if err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.name, err))
		return
	}

	dev := d.Devices[config.address]
	if err := dev.GetControlInfo(); err != nil {
		logger.Fatal(fmt.Sprintf("daikin (%s): %v", config.name, err))
		return
	}

	subControl, _ := bus.Subscribe(fmt.Sprintf("daikin.%s.control", config.name))
	defer subControl.Close()

	for event := range subControl.Ch {
		if event.Key == "power" && event.Value == "off" {
			if err := dev.GetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
				continue
			}

			dev.ControlInfo.Power = daikinClient.PowerOff
			if err := dev.SetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): error setting control: %v", config.name, err))
				continue
			}
		} else if event.Key == "power" && event.Value == "on" {
			if err := dev.GetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
				continue
			}

			dev.ControlInfo.Power = daikinClient.PowerOn
			if err := dev.SetControlInfo(); err != nil {
				logger.Error(fmt.Sprintf("daikin (%s): error setting control: %v", config.name, err))
				continue
			}
		} else {
			logger.Error(fmt.Sprintf("daikin (%s): unrecognised event: %v", config.name, event))
		}
	}
}

func newConfigFromSection(configSection *conf.ConfigSection) (configData, error) {
	name, err := configSection.GetString("name")
	if err != nil {
		return configData{}, fmt.Errorf("name not found in config")
	}

	address, err := configSection.GetString("address")
	if err != nil {
		return configData{}, fmt.Errorf("address not found in config for %s", name)
	}

	token, err := configSection.GetString("token")
	if err != nil {
		token = ""
	}

	return configData{
		address: address,
		name:    name,
		token:   token,
	}, nil
}
