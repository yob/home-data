package ruuvi

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
	pubsub "github.com/yob/home-data/pubsub"
)

type RuuviAdapter struct {
	addressMap     map[string]string
	publishChannel chan pubsub.PubsubEvent
}

func NewRuuviAdapter(publishChannel chan pubsub.PubsubEvent, addressMap map[string]string) *RuuviAdapter {
	return &RuuviAdapter{
		addressMap:     addressMap,
		publishChannel: publishChannel,
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
	}

	fmt.Fprintf(w, "OK")
	return 200, nil
}
