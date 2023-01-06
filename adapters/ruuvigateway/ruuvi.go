package ruuvigateway

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	pubsub "github.com/yob/home-data/pubsub"

	"gitlab.com/jtaimisto/bluewalker/ruuvi"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, config *conf.ConfigSection) {
	ip, err := config.GetString("ip")
	if err != nil {
		logger.Fatal("ruuvigateway: ip not found in config")
		return
	}

	addressMap, err := config.GetStringMap("names")
	if err != nil {
		logger.Fatal(fmt.Sprintf("ruuvigateway: names map not found in config - %v", err))
		return
	}

	for {
		time.Sleep(20 * time.Second)

		fetchBleHistory(bus, logger, state, ip, addressMap)
	}
}

func fetchBleHistory(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, ip string, addressMap map[string]string) {
	ruuviGatewayHistoryUrl := fmt.Sprintf("http://%s/history", ip)

	resp, err := http.Get(ruuviGatewayHistoryUrl)
	defer resp.Body.Close()

	if err != nil {
		logger.Error(fmt.Sprintf("ruuvigateway: %v\n", err))
		return
	}
	if resp.StatusCode != 200 {
		logger.Error(fmt.Sprintf("ruuvigateway: unexpected response code %d\n", resp.StatusCode))
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	jsonBody := buf.String()

	if !gjson.Valid(jsonBody) {
		logger.Error(fmt.Sprintf("ruuvigateway: invalid JSON"))
		return
	}

	resultTags := gjson.Get(jsonBody, "data.tags").Map()
	for mac, macObj := range resultTags {

		macData := macObj.Get("data").String()
		macDataBytes, err := hex.DecodeString(macData)
		if err != nil {
			logger.Error(fmt.Sprintf("ruuvigateway: failed to decode Hex data %v", err))
			continue
		}

		ads, err := parseAdData(macDataBytes)
		if err != nil {
			logger.Error(fmt.Sprintf("ruuvigateway: %+v", err))
			continue
		}
		for _, ad := range ads {
			// ruuvitags send BLE advertisements with a Type of "Manufacturer Specific" and a
			// little endian data payload that starts with "0x99 0x04". For now we're filtering
			// the discovered advertisements down to ruuvi only, but in the future it would be
			// totally possible to and new checks and trigger events for non-ruuvi BLE advertisements
			if ad.typ == adManufacturerSpecific && binary.LittleEndian.Uint16(ad.data) == 0x0499 {
				ruuviData, err := ruuvi.Decode(macDataBytes[7:])
				if err != nil {
					logger.Error(fmt.Sprintf("ruuvigateway: error decoding advertisment (%+v)", err))
					continue
				}

				if ruuviName, ok := addressMap[strings.ToLower(mac)]; ok {
					handleRuuviAd(bus, logger, ruuviName, ruuviData)
				}
			}
		}
	}
}

func handleRuuviAd(bus *pubsub.Pubsub, logger *logging.Logger, ruuviName string, data *ruuvi.Data) {
	tempSensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.temp_celcius", ruuviName))
	humiditySensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.humidity", ruuviName))
	pressureSensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.pressure", ruuviName))
	voltageSensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.voltage", ruuviName))
	txpowerSensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.txpower", ruuviName))
	dewpointSensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.dewpoint_celcius", ruuviName))
	absoluteHumiditySensor := entities.NewSensorGauge(bus, fmt.Sprintf("ruuvi.%s.absolute_humidity_g_per_m3", ruuviName))

	tempSensor.Update(float64(data.Temperature))
	humiditySensor.Update(float64(data.Humidity))
	pressureSensor.Update(float64(data.Pressure))
	voltageSensor.Update(float64(data.Voltage))
	txpowerSensor.Update(float64(data.TxPower))

	dewpoint, err := calculateDewPoint(float64(data.Temperature), float64(data.Humidity))
	if err == nil {
		dewpointSensor.Update(dewpoint)
	} else {
		logger.Error(fmt.Sprintf("ruuvigateway: error calculating dewpoint - %v", err))
	}

	absoluteHumidity, err := calculateAbsoluteHumidity(float64(data.Temperature), float64(data.Humidity))
	if err == nil {
		absoluteHumiditySensor.Update(absoluteHumidity)
	} else {
		logger.Error(fmt.Sprintf("ruuvigateway: error calculating absolute humidity - %v", err))
	}
}

// The formula is from https://carnotcycle.wordpress.com/2012/08/04/how-to-convert-relative-humidity-to-absolute-humidity/
// I've checked the output against the absolute humidity numbers I can see in the ruuvi android app the they're match to
// within a few hundredths of a gram. Good enough?
func calculateAbsoluteHumidity(T float64, H float64) (float64, error) {
	// Check if the transferred value for the temperature is within the valid range
	if T < -45 || T > 60 {
		return 0, errors.New("Temperature must be between (-45 - +60°C)")
	}
	// Check if the transferred value for humidity is within the valid range
	if H < 0 || H > 100 {
		return 0, errors.New("Humidity must be between (0 - 100%)")
	}

	num := 6.112 * math.Exp((17.67*T)/(T+243.5)) * H * 2.1674
	denom := 273.15 + T
	result := (num / denom)

	// return the answer rounded to 2 decimal places, we don't need excessive precision
	return math.Round(result*100) / 100, nil
}

// From https://github.com/de-wax/go-pkg/blob/a5a606b51a6fa86dc0b561d4d019b3d7fc1e479b/dewpoint/dewpoint.go
// The results match what I get when I plug values into http://www.dpcalc.org/, so maybe they're about right?
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

/////////////////////////////////////////////////////////
// Everything belong here is a vendored fragment from "gitlab.com/jtaimisto/bluewalker/hci"
// I'd use that pckage directly, howver the key function I need is private
/////////////////////////////////////////////////////////

// adType is the type for advertising data
// See Bluetooth 5.0, vol 3, part C, ch 11
type adType byte

// AD type values
// See https://www.bluetooth.com/specifications/assigned-numbers/generic-access-profile
const (
	adFlags                  adType = 0x01
	adMore16BitService       adType = 0x02
	adComplete16BitService   adType = 0x03
	adMore32BitService       adType = 0x04
	adComplete32BitService   adType = 0x05
	adMore128BitService      adType = 0x06
	adComplete128BitService  adType = 0x07
	adShortenedLocalName     adType = 0x08
	adCompleteLocalName      adType = 0x09
	adTxPower                adType = 0x0a
	adClassOfdevice          adType = 0x0d
	adPairingHash            adType = 0x0e
	adPairingRandomizer      adType = 0x0f
	adSmTk                   adType = 0x10
	adSmOobFlags             adType = 0x11
	adSlaveConnInterval      adType = 0x12
	ad16bitServiceSol        adType = 0x14
	ad128bitServiceSol       adType = 0x15
	adServiceData            adType = 0x16
	adPublicTargetAddr       adType = 0x17
	adRandomTargetAddr       adType = 0x18
	adAppearance             adType = 0x19
	adAdvInterval            adType = 0x1a
	adDeviceAddress          adType = 0x1b
	adLeRole                 adType = 0x1c
	adPairingHash256         adType = 0x1d
	adPairingRandomizer256   adType = 0x1e
	ad32BitServiceSol        adType = 0x1f
	adServiceData32          adType = 0x20
	adServiceData128         adType = 0x21
	adSecureConnConfirm      adType = 0x22
	adSecureConnRandom       adType = 0x23
	adURI                    adType = 0x24
	adIndoorPosit            adType = 0x25
	adTransportDiscoveryData adType = 0x26
	adLeSupportedFeatures    adType = 0x27
	adChannelMapUpdate       adType = 0x28
	adMeshPbAdv              adType = 0x29
	adMeshMessage            adType = 0x2a
	adMeshBeacon             adType = 0x2b
	ad3dData                 adType = 0x3d
	adManufacturerSpecific   adType = 0xff
)

func (ad adType) String() string {
	switch ad {
	case adFlags:
		return "Flags"
	case adMore16BitService:
		return "16 Bit Service Class UUID"
	case adComplete16BitService:
		return "Complete 16 Bit Service Class UUID"
	case adMore32BitService:
		return "32 Bit Service Class UUID"
	case adComplete32BitService:
		return "Complete 32 Bit Service Class UUID"
	case adMore128BitService:
		return "128 Bit Service Class UUID"
	case adComplete128BitService:
		return "Complete 128 Bit Service Class UUID"
	case adShortenedLocalName:
		return "Shortened Local name"
	case adCompleteLocalName:
		return "Complete local name"
	case adTxPower:
		return "Tx Power"
	case adClassOfdevice:
		return "Class of device"
	case adManufacturerSpecific:
		return "Manufacturer Specific"
	case adDeviceAddress:
		return "LE Bluetooth Device Address"
	case adAppearance:
		return "Appearance"
	case adPairingHash:
		return "Simple Pairing Hash"
	case adPairingRandomizer:
		return "Simple Pairing Randomizer"
	case adSmTk:
		return "Security Manager TK Value"
	case adSmOobFlags:
		return "Security Manager OOB Flags"
	case adSlaveConnInterval:
		return "Slave Connection Interval Range"
	case ad16bitServiceSol:
		return "List of 16-bit Service Solicitation UUIDs"
	case ad128bitServiceSol:
		return "List of 128-bit Service Solicitation UUIDs"
	case adServiceData:
		return "Service Data"
	case adPublicTargetAddr:
		return "Public Target Address"
	case adRandomTargetAddr:
		return "Random Target Address"
	case adAdvInterval:
		return "Advertising interval"
	case adLeRole:
		return "LE Role"
	case adPairingHash256:
		return "Simple Pairing Hash C-256"
	case adPairingRandomizer256:
		return "Simple Pairing Randomizer R-256"
	case ad32BitServiceSol:
		return "List of 32-bit Service Solicitation UUIDs"
	case adServiceData32:
		return "Service Data - 32-bit UUID"
	case adServiceData128:
		return "Service Data - 128-bit UUID"
	case adSecureConnConfirm:
		return "LE Secure Connections Confirmation Value"
	case adSecureConnRandom:
		return "LE Secure Connections Random Value"
	case adURI:
		return "URI"
	case adIndoorPosit:
		return "Indoor Positioning"
	case adTransportDiscoveryData:
		return "Transport Discovery Data"
	case adLeSupportedFeatures:
		return "LE Supported Features"
	case adChannelMapUpdate:
		return "Channel Map Update Indication"
	case adMeshPbAdv:
		return "PB-ADV"
	case adMeshMessage:
		return "Mesh Message"
	case adMeshBeacon:
		return "Mesh Beacon"
	case ad3dData:
		return "3D Data"
	default:
		return fmt.Sprintf("Unknown (%.2x)", int(ad))
	}
}

type adStructure struct {
	typ  adType
	data []byte
}

func (ad *adStructure) String() string {
	return fmt.Sprintf("%s : 0x%x", ad.typ.String(), ad.data)
}

func decodeAdStructure(buf []byte) (*adStructure, error) {
	length := int(buf[0])
	// sometimes zero -length AD structres are used for padding
	if length == 0 {
		return nil, nil
	}
	if length+1 > len(buf) {
		return nil, fmt.Errorf("invalid length for AD Structure")
	}
	t := adType(buf[1])
	dat := buf[2 : 2+length-1]
	return &adStructure{typ: t, data: dat}, nil
}

// ParseAdData parses the advertising data to ad structres
func parseAdData(buf []byte) ([]*adStructure, error) {

	offset := 0
	structures := make([]*adStructure, 0)
	for offset < len(buf) {
		ad, err := decodeAdStructure(buf[offset:])
		if err != nil {
			return nil, err
		}
		if ad == nil {
			// in theory, there could be another structure after
			// 0 -length block.
			offset++
			continue
		}
		structures = append(structures, ad)
		offset += (len(ad.data) + 2)
	}
	return structures, nil
}
