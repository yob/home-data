package daikin

import (
	"fmt"
	"time"

	daikinClient "github.com/buxtronix/go-daikin"
	"github.com/yob/home-data/pubsub"
)

func Poll(bus *pubsub.Pubsub, name string, address string, token string) {
	publish := bus.PublishChannel()
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(address, token))
	if err != nil {
		fatalLog(publish, fmt.Sprintf("daikin (%s): %v", name, err))
		return
	}

	dev := d.Devices[address]
	if err := dev.GetControlInfo(); err != nil {
		fatalLog(publish, fmt.Sprintf("daikin (%s): %v", name, err))
		return
	}

	for {
		time.Sleep(5 * time.Second)

		if err := dev.GetSensorInfo(); err != nil {
			errorLog(publish, fmt.Sprintf("daikin (%s): %v", name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.temp_inside_celcius", name), Value: dev.SensorInfo.HomeTemperature.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.temp_outside_celcius", name), Value: dev.SensorInfo.OutsideTemperature.String()},
		}

		if err := dev.GetControlInfo(); err != nil {
			errorLog(publish, fmt.Sprintf("daikin (%s): %v", name, err))
			continue
		}

		var powerInt = 0 // 0 == Off, 1 == On
		if dev.ControlInfo.Power.String() == "On" {
			powerInt = 1
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.power", name), Value: fmt.Sprintf("%d", powerInt)},
		}

		if err := dev.GetWeekPower(); err != nil {
			errorLog(publish, fmt.Sprintf("daikin (%s): %v", name, err))
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.watt_hours_today", name), Value: dev.WeekPower.TodayWattHours.String()},
		}
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
