package datadog

import (
	"context"
	"fmt"
	"strconv"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	pubsub "github.com/yob/home-data/pubsub"

	datadog "github.com/DataDog/datadog-api-client-go/api/v1/datadog"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, config *conf.ConfigSection) {
	apiKey, err := config.GetString("api_key")
	if err != nil {
		logger.Fatal("datadog: api_key not found in config")
		return
	}
	appKey, err := config.GetString("app_key")
	if err != nil {
		logger.Fatal("datadog: app_key not found in config")
		return
	}

	interestingKeys, err := config.GetStringSlice("keys")
	if err != nil {
		logger.Fatal("datadog: keys not found in config")
		return
	}

	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		processEvent(logger, apiKey, appKey, state, interestingKeys)
	}
}

func processEvent(logger *logging.Logger, apiKey string, appKey string, state memorystate.StateReader, interestingKeys []string) {
	for _, stateKey := range interestingKeys {
		if value, ok := state.Read(stateKey); ok {
			value64, err := strconv.ParseFloat(value, 8)
			if err == nil {
				ddSubmitGauge(logger, apiKey, appKey, stateKey, value64)
			}
		} else {
			logger.Debug(fmt.Sprintf("datadog: failed to read %s from state", stateKey))
		}
	}
}

func ddSubmitGauge(logger *logging.Logger, apiKey string, appKey string, property string, value float64) {
	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {
				Key: apiKey,
			},
			"appKeyAuth": {
				Key: appKey,
			},
		},
	)

	nowEpoch := float64(time.Now().Unix())
	body := *datadog.NewMetricsPayload([]datadog.Series{*datadog.NewSeries(property, [][]float64{[]float64{nowEpoch, value}})})
	configuration := datadog.NewConfiguration()

	apiClient := datadog.NewAPIClient(configuration)
	_, r, err := apiClient.MetricsApi.SubmitMetrics(ctx, body)
	if err != nil {
		logger.Error(fmt.Sprintf("datadog: Error when calling `MetricsApi.SubmitMetrics`: %v", err))
		logger.Error(fmt.Sprintf("datadog: Full HTTP response: %v", r))
		return
	}

	logger.Debug(fmt.Sprintf("datadog: Wrote MetricsApi.SubmitMetrics: %s %v", property, value))
	return
}
