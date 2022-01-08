package lifx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
	"go.yhsif.com/lifxlan"
	"go.yhsif.com/lifxlan/light"
)

type configData struct {
	address string
	name    string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, configSection *conf.ConfigSection) {
	var wg sync.WaitGroup

	config, err := newConfigFromSection(configSection)
	if err != nil {
		logger.Fatal(fmt.Sprintf("lifx (%s): %v", config.name, err))
		return
	}

	wg.Add(1)
	go func() {
		broadcastState(bus, logger, config)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		changeState(bus, logger, config)
		wg.Done()
	}()

	wg.Wait()
}

func broadcastState(bus *pubsub.Pubsub, logger *logging.Logger, config configData) {
	timeout := 2 * time.Second

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	lifxDev := lifxlan.NewDevice(config.address, lifxlan.ServiceUDP, lifxlan.AllDevices)
	lightDev, err := light.Wrap(ctx, lifxDev, false)

	if err != nil {
		logger.Fatal(fmt.Sprintf("lifx (%s): %v", config.name, err))
		return
	}

	colorSensor := entities.NewSensorString(bus, fmt.Sprintf("lifx.%s.color", config.name))

	for {
		time.Sleep(30 * time.Second)

		ctx, _ := context.WithTimeout(context.Background(), timeout)
		color, err := lightDev.GetColor(ctx, nil)
		if err != nil {
			logger.Error(fmt.Sprintf("lifx (%s): error geetting color: %v", config.name, err))
			continue
		}

		colorSensor.Update(serialiseColor(color))
	}
}

func changeState(bus *pubsub.Pubsub, logger *logging.Logger, config configData) {
	timeout := 10 * time.Second

	subControl, _ := bus.Subscribe(fmt.Sprintf("lifx.%s.control", config.name))
	defer subControl.Close()

	for event := range subControl.Ch {
		ctx, _ := context.WithTimeout(context.Background(), timeout)
		lifxDev := lifxlan.NewDevice(config.address, lifxlan.ServiceUDP, lifxlan.AllDevices)
		lightDev, err := light.Wrap(ctx, lifxDev, false)

		if err != nil {
			logger.Fatal(fmt.Sprintf("lifx (%s): error during changeState init: %v", config.name, err))
			continue
		}

		if event.Key == "color:set" {
			color, err := deserialiseColor(event.Value)
			if err != nil {
				logger.Error(fmt.Sprintf("lifx (%s): error setting color: %v", config.name, err))
				continue
			}

			ctx, _ := context.WithTimeout(context.Background(), timeout)
			err = lightDev.SetColor(ctx, nil, color, 0, true)
			if err != nil {
				logger.Error(fmt.Sprintf("lifx (%s): error setting color: %v", config.name, err))
				continue
			}
			logger.Debug(fmt.Sprintf("lifx (%s): color changed", config.name))
		} else {
			logger.Error(fmt.Sprintf("lifx (%s): unrecognised event: %v", config.name, event))
		}
	}
}

func serialiseColor(color *lifxlan.Color) string {
	return fmt.Sprintf("%d,%d,%d,%d", color.Hue, color.Saturation, color.Brightness, color.Kelvin)
}

func deserialiseColor(value string) (*lifxlan.Color, error) {
	if value == "red" {
		return &lifxlan.Color{Hue: 3011, Saturation: 65535, Brightness: 39403, Kelvin: 3500}, nil
	} else if value == "orange" {
		return &lifxlan.Color{Hue: 4480, Saturation: 65535, Brightness: 39403, Kelvin: 3500}, nil
	} else if value == "green" {
		return &lifxlan.Color{Hue: 25197, Saturation: 65535, Brightness: 39403, Kelvin: 3500}, nil
	} else {
		components := strings.Split(value, ",")
		if len(components) == 4 {
			hue, _ := strconv.Atoi(components[0])
			saturation, _ := strconv.Atoi(components[1])
			brightness, _ := strconv.Atoi(components[2])
			kelvin, _ := strconv.Atoi(components[3])
			return &lifxlan.Color{
				Hue:        uint16(hue),
				Saturation: uint16(saturation),
				Brightness: uint16(brightness),
				Kelvin:     uint16(kelvin),
			}, nil
		} else {
			return &lifxlan.Color{}, fmt.Errorf("unable to parse color")
		}
	}
}

func newConfigFromSection(configSection *conf.ConfigSection) (configData, error) {
	name, err := configSection.GetString("name")
	if err != nil {
		return configData{}, fmt.Errorf("name not found in config")
	}

	address, err := configSection.GetString("address")
	if err != nil {
		return configData{}, fmt.Errorf("address not found in config")
	}

	return configData{
		address: address,
		name:    name,
	}, nil
}
