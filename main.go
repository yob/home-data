package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	evbus "github.com/asaskevich/EventBus"
	daikin "github.com/buxtronix/go-daikin"
	"github.com/dim13/unifi"
	"github.com/tidwall/gjson"
)

const (
	kitchen_ip  = "10.1.1.110"
	inverter_ip = "10.1.1.69"
	unifi_ip    = "10.1.1.2"
)

var (
	state = sync.Map{} // map[string]string{}
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

	// fronius plugin, one per inverter
	wg.Add(1)
	go func() {
		froniusInverter(bus, inverter_ip)
		wg.Done()

	}()

	// unifi plugin, one per network to detect presense of specific people
	wg.Add(1)
	go func() {
		unifiPresence(bus, unifi_ip)
		wg.Done()

	}()

	// stackdriver plugin, for sending metrics to google stackdriver
	bus.SubscribeAsync("state:broadcast:minute", stackdriverProcess, false)
	bus.SubscribeAsync("stackdriver:submit:gauge", stackSubmitGauge, false)

	wg.Wait()
}

func unifiPresence(bus evbus.Bus, address string) {

	var ipMap = map[string]string{
		"10.1.1.123": "james",
		"10.1.1.134": "andrea",
	}

	unifi_user := os.Getenv("UNIFI_USER")
	unifi_pass := os.Getenv("UNIFI_PASS")
	unifi_port := os.Getenv("UNIFI_PORT")
	unifi_site := os.Getenv("UNIFI_SITE")
	u, err := unifi.Login(unifi_user, unifi_pass, address, unifi_port, unifi_site, 5)
	if err != nil {
		log.Fatalf("Unifi login returned error: %v\n", err)
	}
	defer u.Logout()

	for {
		site, err := u.Site("default")
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		stations, err := u.Sta(site)
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		for _, s := range stations {
			if stationName, ok := ipMap[s.IP]; ok {
				lastSeen := time.Unix(s.LastSeen, 0).UTC()
				bus.Publish("state:update", fmt.Sprintf("unifi.presence.last_seen.%s", stationName), lastSeen.Format(time.RFC3339))
			}
		}

		time.Sleep(20 * time.Second)
	}
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
		time.Sleep(5 * time.Second)

		if err := dev.GetSensorInfo(); err != nil {
			log.Fatalf("ERROR: %v", err)
		}

		bus.Publish("state:update", "kitchen.daikin.temp_inside_celcius", dev.SensorInfo.HomeTemperature.String())
		bus.Publish("state:update", "kitchen.daikin.temp_outside_celcius", dev.SensorInfo.OutsideTemperature.String())
		bus.Publish("state:update", "kitchen.daikin.humidity", dev.SensorInfo.Humidity.String())
	}
}

func froniusInverter(bus evbus.Bus, address string) {
	powerFlowUrl := fmt.Sprintf("http://%s//solar_api/v1/GetPowerFlowRealtimeData.fcgi", address)
	meterDataUrl := fmt.Sprintf("http://%s//solar_api/v1/GetMeterRealtimeData.cgi?Scope=System", address)

	for {
		time.Sleep(20 * time.Second)

		resp, err := http.Get(powerFlowUrl)
		if err != nil {
			fmt.Printf("ERROR - froniusInverter: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		jsonBody := buf.String()

		gridDrawWatts := gjson.Get(jsonBody, "Body.Data.Site.P_Grid")
		powerWatts := gjson.Get(jsonBody, "Body.Data.Site.P_Load")
		generationWatts := gjson.Get(jsonBody, "Body.Data.Site.P_PV")
		energyDayWh := gjson.Get(jsonBody, "Body.Data.Site.E_Day")

		bus.Publish("state:update", "fronius.inverter.grid_draw_watts", gridDrawWatts.String())
		bus.Publish("state:update", "fronius.inverter.power_watts", powerWatts.String())
		bus.Publish("state:update", "fronius.inverter.generation_watts", generationWatts.String())
		bus.Publish("state:update", "fronius.inverter.energy_day_watt_hours", energyDayWh.String())

		resp, err = http.Get(meterDataUrl)
		if err != nil {
			fmt.Printf("ERROR - froniusInverter: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		buf = new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		jsonBody = buf.String()

		gridVoltage := gjson.Get(jsonBody, "Body.Data.0.Voltage_AC_Phase_1")
		bus.Publish("state:update", "fronius.inverter.grid_voltage", gridVoltage.String())
	}
}

func stackdriverProcess(localState sync.Map) {
	if value, ok := localState.Load("kitchen.daikin.temp_inside_celcius"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			bus.Publish("stackdriver:submit:gauge", "kitchen.daikin.temp_inside_celcius", value64)
		}
	}

	if value, ok := localState.Load("kitchen.daikin.temp_outside_celcius"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			bus.Publish("stackdriver:submit:gauge", "kitchen.daikin.temp_outside_celcius", value64)
		}
	}

	if value, ok := localState.Load("kitchen.daikin.humidity"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			bus.Publish("stackdriver:submit:gauge", "kitchen.daikin.humidity", value64)
		}
	}
}

func stackSubmitGauge(property string, value float64) {
	fmt.Printf("TODO: submit to stackdriver %s %v\n", property, value)
}

func stateUpdate(property string, value string) {
	existingValue, ok := state.Load(property)

	// if the property doesn't exist in the state yet, or it exists with a different value, then update it
	if !ok || existingValue != value {
		state.Store(property, value)
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
