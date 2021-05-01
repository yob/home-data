package daikin

import (
	"fmt"
	"log"
	"time"

	daikinClient "github.com/buxtronix/go-daikin"
	pubsub "github.com/yob/home-data/pubsub"
)

func Poll(publish chan pubsub.PubsubEvent, name string, address string, token string) {
	d, err := daikinClient.NewNetwork(daikinClient.AddressTokenOption(address, token))
	if err != nil {
		fmt.Printf("ERROR: %v", err)
		return
	}

	dev := d.Devices[address]
	if err := dev.GetControlInfo(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	for {
		time.Sleep(5 * time.Second)

		if err := dev.GetSensorInfo(); err != nil {
			log.Printf("ERROR daikin: %v\n", err)
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
			log.Printf("ERROR daikin: %v\n", err)
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
			log.Printf("ERROR daikin: %v\n", err)
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("daikin.%s.watt_hours_today", name), Value: dev.WeekPower.TodayWattHours.String()},
		}
	}
}
