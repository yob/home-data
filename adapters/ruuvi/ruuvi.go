package ruuvi

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"

	"github.com/tidwall/gjson"
	pubsub "github.com/yob/home-data/pubsub"
)

type RuuviAdapter struct {
	addressMap     map[string]string
	publishChannel chan pubsub.PubsubEvent
}

func NewRuuviAdapter(bus *pubsub.Pubsub, addressMap map[string]string) *RuuviAdapter {
	return &RuuviAdapter{
		addressMap:     addressMap,
		publishChannel: bus.PublishChannel(),
	}
}

func (adapter *RuuviAdapter) HttpHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return 404, nil
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

	if ruuviName, ok := adapter.addressMap[device_mac.String()]; ok {
		temp := gjson.Get(jsonBody, "sensors.temperature")
		humidity := gjson.Get(jsonBody, "sensors.humidity")
		pressure := gjson.Get(jsonBody, "sensors.pressure")
		voltage := gjson.Get(jsonBody, "sensors.voltage")
		txpower := gjson.Get(jsonBody, "sensors.txpower")

		adapter.publishChannel <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.temp_celcius", ruuviName), Value: temp.String()},
		}
		adapter.publishChannel <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.humidity", ruuviName), Value: humidity.String()},
		}
		adapter.publishChannel <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.pressure", ruuviName), Value: pressure.String()},
		}
		adapter.publishChannel <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.voltage", ruuviName), Value: voltage.String()},
		}
		adapter.publishChannel <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.txpower", ruuviName), Value: txpower.String()},
		}

		dewpoint, err := calculateDewPoint(temp.Float(), humidity.Float())
		if err == nil {
			adapter.publishChannel <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.dewpoint_celcius", ruuviName), Value: strconv.FormatFloat(dewpoint, 'f', -1, 64)},
			}
		} else {
			errorLog(adapter.publishChannel, fmt.Sprintf("ruuvi: error calculating dewpoint - %v", err))
		}
	}

	fmt.Fprintf(w, "OK")
	return 200, nil
}

// From https://github.com/de-wax/go-pkg/blob/a5a606b51a6fa86dc0b561d4d019b3d7fc1e479b/dewpoint/dewpoint.go
func calculateDewPoint(T float64, H float64) (float64, error) {
	// Check if the transferred value for the temperature is within the valid range
	if T < -45 || T > 60 {
		return 0, errors.New("Temperature must be between (-45 - +60°C)")
	}
	// Check if the transferred value for humidity is within the valid range
	if H < 0 || H > 100 {
		return 0, errors.New("Humidity must be between (0 - 100%)")
	}

	// Constants for the Magnus formula
	const a float64 = 17.62
	const b float64 = 243.12

	// Magnus formula
	alpha := math.Log(H/100) + a*T/(b+T)
	return math.Round(((b*alpha)/(a-alpha))*100) / 100, nil
}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "ERROR", Value: message},
	}

}
