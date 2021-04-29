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
			log.Fatalf("ERROR: %v", err)
		}

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("%s.daikin.temp_inside_celcius", name), Value: dev.SensorInfo.HomeTemperature.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("%s.daikin.temp_outside_celcius", name), Value: dev.SensorInfo.OutsideTemperature.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("%s.daikin.humidity", name), Value: dev.SensorInfo.Humidity.String()},
		}
	}
}
