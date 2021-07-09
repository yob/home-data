package datadog

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/yob/home-data/core/logging"
	pubsub "github.com/yob/home-data/pubsub"

	datadog "github.com/DataDog/datadog-api-client-go/api/v1/datadog"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, localState *sync.Map, interestingKeys []string) {
	apiKey := os.Getenv("DD_API_KEY")
	if apiKey == "" {
		logger.Fatal("env var DD_API_KEY must be set for metrics to be submitted to datadog")
		return
	}
	appKey := os.Getenv("DD_APP_KEY")
	if appKey == "" {
		logger.Fatal("env var DD_APP_KEY must be set for metrics to be submitted to datadog")
		return
	}

	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		processEvent(logger, localState, interestingKeys)
	}
}

func processEvent(logger *logging.Logger, localState *sync.Map, interestingKeys []string) {
	for _, stateKey := range interestingKeys {
		if value, ok := localState.Load(stateKey); ok {
			value64, err := strconv.ParseFloat(value.(string), 8)
			if err == nil {
				ddSubmitGauge(logger, stateKey, value64)
			}
		} else {
			logger.Debug(fmt.Sprintf("datadog: failed to read %s from state", stateKey))
		}
	}
}

func ddSubmitGauge(logger *logging.Logger, property string, value float64) {
	ctx := datadog.NewDefaultContext(context.Background())

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
