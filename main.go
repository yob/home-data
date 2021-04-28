package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	daikin "github.com/buxtronix/go-daikin"
	"github.com/dim13/unifi"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/tidwall/gjson"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

const (
	kitchen_ip      = "10.1.1.110"
	lounge_ip       = "10.1.1.111"
	study_ip        = "10.1.1.112"
	inverter_ip     = "10.1.1.69"
	unifi_ip        = "10.1.1.2"
	googleProjectID = "our-house-data"
)

var (
	state  = sync.Map{} // map[string]string{}
	pubsub = NewPubsub()
)

type appHandler func(http.ResponseWriter, *http.Request, chan PubsubEvent) (int, error)

func main() {

	// update the shared state when attributes change
	go func() {
		ch_state_update := pubsub.Subscribe("state:update")
		for elem := range ch_state_update {
			stateUpdate(elem.key, elem.value)
		}
	}()

	// send data to stack driver every minute
	go func() {
		ch_every_minute := pubsub.Subscribe("every:minute")
		for _ = range ch_every_minute {
			stackdriverProcess(state)
		}
	}()

	// trigger an event that anyone can listen to if they want to run code every minute
	go func() {
		everyMinuteEvent(pubsub.PublishChannel())
	}()

	// daikin plugin, one per unit
	go func() {
		pollDaikin(pubsub.PublishChannel(), "kitchen", kitchen_ip, "")
	}()
	go func() {
		pollDaikin(pubsub.PublishChannel(), "study", study_ip, os.Getenv("DAIKIN_STUDY_TOKEN"))
	}()
	go func() {
		pollDaikin(pubsub.PublishChannel(), "lounge", lounge_ip, os.Getenv("DAIKIN_LOUNGE_TOKEN"))
	}()

	// fronius plugin, one per inverter
	go func() {
		froniusInverter(pubsub.PublishChannel(), inverter_ip)
	}()

	// unifi plugin, one per network to detect presense of specific people
	go func() {
		unifiPresence(pubsub.PublishChannel(), unifi_ip)
	}()

	// webserver, as an alternative way to injest events
	go func() {
		startHttpServer()
	}()

	// loop forever, shuffling events between goroutines
	pubsub.Run()
}

func startHttpServer() {
	http.Handle("/ruuvi", appHandler(ruuviHttpHandler))
	http.HandleFunc("/", http.NotFound)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if status, err := fn(w, r, pubsub.PublishChannel()); err != nil {
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

func ruuviHttpHandler(w http.ResponseWriter, r *http.Request, publish chan PubsubEvent) (int, error) {
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

		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("ruuvi.%s.temp_celcius", ruuviName), value: temp.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("ruuvi.%s.humidity", ruuviName), value: humidity.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("ruuvi.%s.pressure", ruuviName), value: pressure.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("ruuvi.%s.voltage", ruuviName), value: voltage.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("ruuvi.%s.txpower", ruuviName), value: txpower.String()},
		}
	}

	fmt.Fprintf(w, "OK")
	return 200, nil
}

func unifiPresence(publish chan PubsubEvent, address string) {

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
				publish <- PubsubEvent{
					topic: "state:update",
					data:  KeyValueData{key: fmt.Sprintf("unifi.presence.last_seen.%s", stationName), value: lastSeen.Format(time.RFC3339)},
				}
			}
		}

		time.Sleep(20 * time.Second)
	}
}

func pollDaikin(publish chan PubsubEvent, name string, address string, token string) {
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

		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("%s.daikin.temp_inside_celcius", name), value: dev.SensorInfo.HomeTemperature.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("%s.daikin.temp_outside_celcius", name), value: dev.SensorInfo.OutsideTemperature.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: fmt.Sprintf("%s.daikin.humidity", name), value: dev.SensorInfo.Humidity.String()},
		}
	}
}

func froniusInverter(publish chan PubsubEvent, address string) {
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

		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: "fronius.inverter.grid_draw_watts", value: gridDrawWatts.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: "fronius.inverter.power_watts", value: powerWatts.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: "fronius.inverter.generation_watts", value: generationWatts.String()},
		}
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: "fronius.inverter.energy_day_watt_hours", value: energyDayWh.String()},
		}

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
		publish <- PubsubEvent{
			topic: "state:update",
			data:  KeyValueData{key: "fronius.inverter.grid_voltage", value: gridVoltage.String()},
		}
	}
}

// TODO submit more gauges. Maybe we need a config file or something to list them?
func stackdriverProcess(localState sync.Map) {
	fmt.Printf("in stackdriverProcess\n")
	if value, ok := localState.Load("kitchen.daikin.temp_inside_celcius"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			stackSubmitGauge("kitchen.daikin.temp_inside_celcius", value64)
		}
	} else {
		fmt.Printf("- else\n")
	}

	if value, ok := localState.Load("kitchen.daikin.temp_outside_celcius"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			stackSubmitGauge("kitchen.daikin.temp_outside_celcius", value64)
		}
	}
}

func stackSubmitGauge(property string, value float64) {
	metricType := fmt.Sprintf("custom.googleapis.com/%s", property)
	ctx := context.Background()
	c, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		fmt.Printf("ERROR: %v", err)
		return
	}
	now := &timestamp.Timestamp{
		Seconds: time.Now().Unix(),
	}
	req := &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + googleProjectID,
		TimeSeries: []*monitoringpb.TimeSeries{{
			Metric: &metricpb.Metric{
				Type: metricType,
			},
			Resource: &monitoredrespb.MonitoredResource{
				Type: "global",
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{
						DoubleValue: value,
					},
				},
			}},
		}},
	}
	fmt.Printf("Wrote metric to stackdriver: %+v\n", req)

	err = c.CreateTimeSeries(ctx, req)
	if err != nil {
		fmt.Printf("could not write time series value, %v ", err)
		return
	}
	return
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

func everyMinuteEvent(publish chan PubsubEvent) {
	lastBroadcast := time.Now()

	for {
		if time.Now().After(lastBroadcast.Add(time.Second * 60)) {
			publish <- PubsubEvent{
				topic: "every:minute",
				data:  KeyValueData{key: "now", value: time.Now().Format(time.RFC3339)},
			}
			lastBroadcast = time.Now()
		}
		time.Sleep(1 * time.Second)
	}
}
