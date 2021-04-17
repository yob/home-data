package main

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	evbus "github.com/asaskevich/EventBus"
	daikin "github.com/buxtronix/go-daikin"
)

const (
	kitchen_ip = "10.1.1.110"
)

var (
	state = map[string]string{}
	bus   = evbus.New()
)

func main() {
	var wg sync.WaitGroup

	bus.SubscribeAsync("state:update", stateUpdate, false)

	wg.Add(1)
	go func() {
		perodicStateBroadcast(bus)
		wg.Done()

	}()

	// daikin plugin, one per unit
	wg.Add(1)
	go func() {
		kitchenDaikin(bus, kitchen_ip)
		wg.Done()

	}()

	// stackdriver plugin, for sending metrics to google stackdriver
	bus.SubscribeAsync("state:broadcast:minute", stackdriverProcess, false)
	bus.SubscribeAsync("stackdriver:submit:gauge", stackSubmitGauge, false)

	wg.Wait()
}

func kitchenDaikin(bus evbus.Bus, address string) {

	d, err := daikin.NewNetwork(daikin.AddressOption(kitchen_ip))
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	dev := d.Devices[kitchen_ip]
	if err := dev.GetControlInfo(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	for {
		if err := dev.GetSensorInfo(); err != nil {
			log.Fatalf("ERROR: %v", err)
		}

		bus.Publish("state:update", "kitchen.daikin.temp_inside_celcius", dev.SensorInfo.HomeTemperature.String())
		bus.Publish("state:update", "kitchen.daikin.temp_outside_celcius", dev.SensorInfo.OutsideTemperature.String())
		bus.Publish("state:update", "kitchen.daikin.humidity", dev.SensorInfo.Humidity.String())

		time.Sleep(5 * time.Second)
	}
}

func stackdriverProcess(localState map[string]string) {
	if value, ok := localState["kitchen.daikin.temp_inside_celcius"]; ok {
		value64, err := strconv.ParseFloat(value, 8)
		if err == nil {
			bus.Publish("stackdriver:submit:gauge", "kitchen.daikin.temp_inside_celcius", value64)
		}
	}

	if value, ok := localState["kitchen.daikin.temp_outside_celcius"]; ok {
		value64, err := strconv.ParseFloat(value, 8)
		if err == nil {
			bus.Publish("stackdriver:submit:gauge", "kitchen.daikin.temp_outside_celcius", value64)
		}
	}

	if value, ok := localState["kitchen.daikin.humidity"]; ok {
		value64, err := strconv.ParseFloat(value, 8)
		if err == nil {
			bus.Publish("stackdriver:submit:gauge", "kitchen.daikin.humidity", value64)
		}
	}
}

func stackSubmitGauge(property string, value float64) {
	fmt.Printf("TODO: submit to stackdriver %s %v\n", property, value)
}

func stateUpdate(property string, value string) {
	existingValue, ok := state[property]

	// if the property doesn't exist in the state yet, or it exists with a different value, then update it
	if !ok || existingValue != value {
		state[property] = value
		fmt.Printf("set %s to %s\n", property, value)
	} else {
		//fmt.Printf("Skipped updating state for %s, no change in value\n", property)
	}
}

func perodicStateBroadcast(bus evbus.Bus) {
	lastBroadcast := time.Now()

	for {
		if time.Now().After(lastBroadcast.Add(time.Second * 60)) {
			bus.Publish("state:broadcast:minute", state)
			lastBroadcast = time.Now()
		}
		time.Sleep(1 * time.Second)
	}
}
