package fronius

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
	pubsub "github.com/yob/home-data/pubsub"
)

func Poll(publish chan pubsub.PubsubEvent, address string) {
	powerFlowUrl := fmt.Sprintf("http://%s//solar_api/v1/GetPowerFlowRealtimeData.fcgi", address)
	meterDataUrl := fmt.Sprintf("http://%s//solar_api/v1/GetMeterRealtimeData.cgi?Scope=System", address)

	for {
		time.Sleep(20 * time.Second)

		resp, err := http.Get(powerFlowUrl)
		if err != nil {
			errorLog(publish, fmt.Sprintf("froniusInverter: %v\n", err))
			continue
		}
		defer resp.Body.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		jsonBody := buf.String()

		gridDrawWatts := gjson.Get(jsonBody, "Body.Data.Site.P_Grid")
		// the type shenanigans are ugly, but P_Load comes back as negative and I find it more intuitive
		// to report it as positive number. "How many watts is the site using right now"
		powerWatts := math.Abs(gjson.Get(jsonBody, "Body.Data.Site.P_Load").Float())
		generationWatts := gjson.Get(jsonBody, "Body.Data.Site.P_PV")
		energyDayWh := gjson.Get(jsonBody, "Body.Data.Site.E_Day")

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: "fronius.inverter.grid_draw_watts", Value: gridDrawWatts.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: "fronius.inverter.power_watts", Value: strconv.FormatFloat(powerWatts, 'f', -1, 64)},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: "fronius.inverter.generation_watts", Value: generationWatts.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: "fronius.inverter.energy_day_watt_hours", Value: energyDayWh.String()},
		}

		resp, err = http.Get(meterDataUrl)
		if err != nil {
			errorLog(publish, fmt.Sprintf("froniusInverter: %v\n", err))
			continue
		}
		defer resp.Body.Close()

		buf = new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		jsonBody = buf.String()

		gridVoltage := gjson.Get(jsonBody, "Body.Data.0.Voltage_AC_Phase_1")
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: "fronius.inverter.grid_voltage", Value: gridVoltage.String()},
		}
	}
}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "ERROR", Value: message},
	}

}
