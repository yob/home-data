package fronius

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	pubsub "github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, config *conf.ConfigSection) {
	address, err := config.GetString("address")
	if err != nil {
		logger.Fatal("fronius: address not found in config")
		return
	}

	powerFlowUrl := fmt.Sprintf("http://%s//solar_api/v1/GetPowerFlowRealtimeData.fcgi", address)
	meterDataUrl := fmt.Sprintf("http://%s//solar_api/v1/GetMeterRealtimeData.cgi?Scope=System", address)

	gridDrawWattsSensor := entities.NewSensorGauge(bus, "fronius.inverter.grid_draw_watts")
	powerWattsSensor := entities.NewSensorGauge(bus, "fronius.inverter.power_watts")
	generationWattsSensor := entities.NewSensorGauge(bus, "fronius.inverter.generation_watts")
	energyDayWhSensor := entities.NewSensorGauge(bus, "fronius.inverter.energy_day_watt_hours")
	gridVoltageSensor := entities.NewSensorGauge(bus, "fronius.inverter.grid_voltage")

	for {
		time.Sleep(20 * time.Second)

		resp, err := http.Get(powerFlowUrl)
		if err != nil {
			logger.Error(fmt.Sprintf("froniusInverter: %v\n", err))
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			logger.Error(fmt.Sprintf("froniusInverter: unexpected response code %d\n", resp.StatusCode))
			continue
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		jsonBody := buf.String()

		gridDrawWatts := gjson.Get(jsonBody, "Body.Data.Site.P_Grid")
		// the type shenanigans are ugly, but P_Load comes back as negative and I find it more intuitive
		// to report it as positive number. "How many watts is the site using right now"
		powerWatts := math.Abs(gjson.Get(jsonBody, "Body.Data.Site.P_Load").Float())
		generationWatts := gjson.Get(jsonBody, "Body.Data.Site.P_PV")
		energyDayWh := gjson.Get(jsonBody, "Body.Data.Site.E_Day")

		gridDrawWattsSensor.Update(gridDrawWatts.Float())
		powerWattsSensor.Update(powerWatts)
		generationWattsSensor.Update(generationWatts.Float())
		energyDayWhSensor.Update(energyDayWh.Float())

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
		gridVoltageSensor.Update(gridVoltage.Float())
	}
}
