package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
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
	lounge_ip   = "10.1.1.111"
	study_ip    = "10.1.1.112"
	inverter_ip = "10.1.1.69"
	unifi_ip    = "10.1.1.2"
)

var (
	state = sync.Map{} // map[string]string{}
	bus   = evbus.New()
)

type appHandler func(http.ResponseWriter, *http.Request) (int, error)

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
		pollDaikin(bus, "kitchen", kitchen_ip, "")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		pollDaikin(bus, "study", study_ip, os.Getenv("DAIKIN_STUDY_TOKEN"))
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		pollDaikin(bus, "lounge", lounge_ip, os.Getenv("DAIKIN_LOUNGE_TOKEN"))
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

	// webserver, as an alternative way to injest events
	wg.Add(1)
	go func() {
		startHttpServer(bus)
		wg.Done()

	}()

	// stackdriver plugin, for sending metrics to google stackdriver
	bus.SubscribeAsync("state:broadcast:minute", stackdriverProcess, false)
	bus.SubscribeAsync("stackdriver:submit:gauge", stackSubmitGauge, false)

	wg.Wait()
}

func startHttpServer(bus evbus.Bus) {
	http.Handle("/ruuvi", appHandler(ruuviHttpHandler))
	http.HandleFunc("/", http.NotFound)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if status, err := fn(w, r); err != nil {
		switch status {
		case http.StatusBadRequest:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case http.StatusNotFound:
			http.NotFound(w, r)
		case http.StatusInternalServerError:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			// Catch any other errors we haven't explicitly handled
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func ruuviHttpHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return 404, nil
	}

	var addressMap = map[string]string{
		"cc:64:a6:ed:f6:aa": "study",
		"f2:b0:81:51:8a:e0": "bed1",
		"fb:dd:03:59:e8:26": "bed2",
		"ef:81:7d:23:3c:74": "lounge",
		"c2:69:9e:be:25:aa": "kitchen",
		"fd:54:a9:f0:a8:a5": "outside",
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 400, fmt.Errorf("ERR: %v", err)
	}
	jsonBody := string(body)
	if !gjson.Valid(jsonBody) {
		return 400, fmt.Errorf("invalid JSON")
	}

	device_mac := gjson.Get(jsonBody, "device.address")

	if ruuviName, ok := addressMap[device_mac.String()]; ok {
		temp := gjson.Get(jsonBody, "sensors.temperature")
		humidity := gjson.Get(jsonBody, "sensors.humidity")
		pressure := gjson.Get(jsonBody, "sensors.pressure")
		voltage := gjson.Get(jsonBody, "sensors.voltage")
		txpower := gjson.Get(jsonBody, "sensors.txpower")

		bus.Publish("state:update", fmt.Sprintf("ruuvi.%s.temp_celcius", ruuviName), temp.String())
		bus.Publish("state:update", fmt.Sprintf("ruuvi.%s.humidity", ruuviName), humidity.String())
		bus.Publish("state:update", fmt.Sprintf("ruuvi.%s.pressure", ruuviName), pressure.String())
		bus.Publish("state:update", fmt.Sprintf("ruuvi.%s.voltage", ruuviName), voltage.String())
		bus.Publish("state:update", fmt.Sprintf("ruuvi.%s.txpower", ruuviName), txpower.String())
	}

	fmt.Fprintf(w, "OK")
	return 200, nil
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

func pollDaikin(bus evbus.Bus, name string, address string, token string) {
	d, err := daikin.NewNetwork(daikin.AddressTokenOption(address, token))
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

		bus.Publish("state:update", fmt.Sprintf("%s.daikin.temp_inside_celcius", name), dev.SensorInfo.HomeTemperature.String())
		bus.Publish("state:update", fmt.Sprintf("%s.daikin.temp_outside_celcius", name), dev.SensorInfo.OutsideTemperature.String())
		bus.Publish("state:update", fmt.Sprintf("%s.daikin.humidity", name), dev.SensorInfo.Humidity.String())
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
