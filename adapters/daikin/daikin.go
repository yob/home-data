package daikin

import (
	"fmt"
	"sync"
	"time"

	daikinClient "github.com/buxtronix/go-daikin"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

type configData struct {
	address string
	name    string
	token   string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, configSection *conf.ConfigSection) {
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
	insideTempSensor := entities.NewSensorGauge(bus, fmt.Sprintf("daikin.%s.temp_inside_celcius", config.name))
	outsideTempSensor := entities.NewSensorGauge(bus, fmt.Sprintf("daikin.%s.temp_outside_celcius", config.name))
	powerSensor := entities.NewSensorBoolean(bus, fmt.Sprintf("daikin.%s.power", config.name))
	wattHoursTodaySensor := entities.NewSensorGauge(bus, fmt.Sprintf("daikin.%s.watt_hours_today", config.name))

	for {
		time.Sleep(20 * time.Second)

		d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.address, config.token))
		if err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		dev := d.Devices[config.address]
		if err := dev.GetControlInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		if err := dev.GetSensorInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		insideTempSensor.Update(float64(dev.SensorInfo.HomeTemperature))
		outsideTempSensor.Update(float64(dev.SensorInfo.OutsideTemperature))

		if err := dev.GetControlInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		powerSensor.Update(dev.ControlInfo.Power.String() == "On")

		if err := dev.GetWeekPower(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		wattHoursTodaySensor.Update(float64(dev.WeekPower.TodayWattHours))
	}
}

func changeState(bus *pubsub.Pubsub, logger *logging.Logger, config configData) {
	for {
		time.Sleep(20 * time.Second)

		d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.address, config.token))
		if err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
		}

		dev := d.Devices[config.address]
		if err := dev.GetControlInfo(); err != nil {
			logger.Error(fmt.Sprintf("daikin (%s): %v", config.name, err))
			continue
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
				logger.Debug(fmt.Sprintf("daikin (%s): power changed to off", config.name))
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
				logger.Debug(fmt.Sprintf("daikin (%s): power changed to on", config.name))
			} else {
				logger.Error(fmt.Sprintf("daikin (%s): unrecognised event: %v", config.name, event))
			}
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
