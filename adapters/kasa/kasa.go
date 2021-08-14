package kasa

import (
	"fmt"
	"sync"
	"time"

	"github.com/jaedle/golang-tplink-hs100/pkg/configuration"
	"github.com/jaedle/golang-tplink-hs100/pkg/hs100"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

type configData struct {
	address string
	name    string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, configSection *conf.ConfigSection) {
	var wg sync.WaitGroup

	config, err := newConfigFromSection(configSection)
	if err != nil {
		logger.Fatal(fmt.Sprintf("kasa: %v", err))
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
	dev := hs100.NewHs100(config.address, configuration.Default())

	_, err := dev.GetName()
	if err != nil {
		logger.Fatal(fmt.Sprintf("kasa (%s): %v", config.name, err))
		return
	}

	powerSensor := entities.NewSensorBoolean(bus, fmt.Sprintf("kasa.%s.on", config.name))

	for {
		time.Sleep(20 * time.Second)

		on, err := dev.IsOn()
		if err != nil {
			logger.Error(fmt.Sprintf("kasa (%s): %v", config.name, err))
			continue
		}

		powerSensor.Update(on)
	}
}

func changeState(bus *pubsub.Pubsub, logger *logging.Logger, config configData) {
	dev := hs100.NewHs100(config.address, configuration.Default())

	_, err := dev.GetName()
	if err != nil {
		logger.Fatal(fmt.Sprintf("kasa (%s): %v", config.name, err))
		return
	}

	subControl, _ := bus.Subscribe(fmt.Sprintf("kasa.%s.control", config.name))
	defer subControl.Close()

	for event := range subControl.Ch {
		if event.Key == "power" && event.Value == "off" {
			err = dev.TurnOff()
			if err != nil {
				logger.Error(fmt.Sprintf("kasa (%s): error setting power to off: %v", config.name, err))
				continue
			}
			logger.Debug(fmt.Sprintf("kasa (%s): power changed to off", config.name))
		} else if event.Key == "power" && event.Value == "on" {
			err = dev.TurnOn()
			if err != nil {
				logger.Error(fmt.Sprintf("kasa (%s): error setting power to on: %v", config.name, err))
				continue
			}
			logger.Debug(fmt.Sprintf("kasa (%s): power changed to on", config.name))
		} else {
			logger.Error(fmt.Sprintf("kasa (%s): unrecognised event: %v", config.name, event))
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

	return configData{
		address: address,
		name:    name,
	}, nil
}
