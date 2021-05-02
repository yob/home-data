package daikin

import (
	"fmt"
	"time"

	daikinClient "github.com/buxtronix/go-daikin"
	"github.com/yob/home-data/pubsub"
)

type Config struct {
	Address string
	Name    string
	Token   string
}

func Poll(bus *pubsub.Pubsub, config Config) {
	publish := bus.PublishChannel()
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(config.Address, config.Token))
	if err != nil {
		fatalLog(publish, fmt.Sprintf("daikin (%s): %v", config.Name, err))
		return
	}

	dev := d.Devices[config.Address]
	if err := dev.GetControlInfo(); err != nil {
		fatalLog(publish, fmt.Sprintf("daikin (%s): %v", config.Name, err))
		return
	}

	for {
		if err := dev.GetSensorInfo(); err != nil {
			errorLog(publish, fmt.Sprintf("daikin (%s): %v", config.Name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.temp_inside_celcius", config.Name), Value: dev.SensorInfo.HomeTemperature.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.temp_outside_celcius", config.Name), Value: dev.SensorInfo.OutsideTemperature.String()},
		}

		if err := dev.GetControlInfo(); err != nil {
			errorLog(publish, fmt.Sprintf("daikin (%s): %v", config.Name, err))
			continue
		}

		var powerInt = 0 // 0 == Off, 1 == On
		if dev.ControlInfo.Power.String() == "On" {
			powerInt = 1
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.power", config.Name), Value: fmt.Sprintf("%d", powerInt)},
		}

		if err := dev.GetWeekPower(); err != nil {
			errorLog(publish, fmt.Sprintf("daikin (%s): %v", config.Name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.watt_hours_today", config.Name), Value: dev.WeekPower.TodayWattHours.String()},
		}

		time.Sleep(20 * time.Second)
	}
}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "ERROR", Value: message},
	}

}

func fatalLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "FATAL", Value: message},
	}

}
