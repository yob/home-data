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

	for {
		time.Sleep(20 * time.Second)

		fetchPowerFlow(bus, logger, state, address)
		fetchMeterData(bus, logger, state, address)
	}
}

func fetchPowerFlow(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, address string) {
	powerFlowUrl := fmt.Sprintf("http://%s/solar_api/v1/GetPowerFlowRealtimeData.fcgi", address)

	gridDrawWattsSensor := entities.NewSensorGauge(bus, "fronius.inverter.grid_draw_watts")
	powerWattsSensor := entities.NewSensorGauge(bus, "fronius.inverter.power_watts")
	generationWattsSensor := entities.NewSensorGauge(bus, "fronius.inverter.generation_watts")
	energyDayWhSensor := entities.NewSensorGauge(bus, "fronius.inverter.energy_day_watt_hours")

	resp, err := http.Get(powerFlowUrl)
	defer resp.Body.Close()

	if err != nil {
		logger.Error(fmt.Sprintf("froniusInverter: %v\n", err))
		return
	}
	if resp.StatusCode != 200 {
		logger.Error(fmt.Sprintf("froniusInverter: unexpected response code %d\n", resp.StatusCode))
		return
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
}

func fetchMeterData(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, address string) {
	meterDataUrl := fmt.Sprintf("http://%s/solar_api/v1/GetMeterRealtimeData.cgi?Scope=System", address)

	gridVoltageSensor := entities.NewSensorGauge(bus, "fronius.inverter.grid_voltage")
	consumedKwHSensor := entities.NewSensorGauge(bus, "fronius.inverter.consumed_kwh")

	resp, err := http.Get(meterDataUrl)
	if err != nil {
		logger.Error(fmt.Sprintf("froniusInverter: %v\n", err))
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	jsonBody := buf.String()

	// now that we've read the body, close it
	resp.Body.Close()

	gridVoltage := gjson.Get(jsonBody, "Body.Data.0.Voltage_AC_Phase_1")
	gridVoltageSensor.Update(gridVoltage.Float())

	consumedKwH := gjson.Get(jsonBody, "Body.Data.0.EnergyReal_WAC_Sum_Consumed")
	consumedKwHSensor.Update(consumedKwH.Float() / 1000.0)
}
