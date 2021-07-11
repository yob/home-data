package fronius

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	pubsub "github.com/yob/home-data/pubsub"
)

type Config struct {
	Address string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, config Config) {
	publish := bus.PublishChannel()
	powerFlowUrl := fmt.Sprintf("http://%s//solar_api/v1/GetPowerFlowRealtimeData.fcgi", config.Address)
	meterDataUrl := fmt.Sprintf("http://%s//solar_api/v1/GetMeterRealtimeData.cgi?Scope=System", config.Address)

	for {
		time.Sleep(20 * time.Second)

		resp, err := http.Get(powerFlowUrl)
		if err != nil {
			logger.Error(fmt.Sprintf("froniusInverter: %v\n", err))
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
			Data:  pubsub.NewKeyValueEvent("fronius.inverter.grid_draw_watts", gridDrawWatts.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent("fronius.inverter.power_watts", strconv.FormatFloat(powerWatts, 'f', -1, 64)),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent("fronius.inverter.generation_watts", generationWatts.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent("fronius.inverter.energy_day_watt_hours", energyDayWh.String()),
		}

		resp, err = http.Get(meterDataUrl)
		if err != nil {
			logger.Error(fmt.Sprintf("froniusInverter: %v\n", err))
			continue
		}
		defer resp.Body.Close()

		buf = new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		jsonBody = buf.String()

		gridVoltage := gjson.Get(jsonBody, "Body.Data.0.Voltage_AC_Phase_1")
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent("fronius.inverter.grid_voltage", gridVoltage.String()),
		}
	}
}
