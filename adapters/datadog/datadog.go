package datadog

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	pubsub "github.com/yob/home-data/pubsub"

	datadog "github.com/DataDog/datadog-api-client-go/api/v1/datadog"
)

func Init(bus *pubsub.Pubsub, localState *sync.Map, interestingKeys []string) {
	apiKey := os.Getenv("DD_API_KEY")
	if apiKey == "" {
		errorLog(bus.PublishChannel(), "env var DD_API_KEY must be set for metrics to be submitted to datadog")
		return
	}
	appKey := os.Getenv("DD_APP_KEY")
	if appKey == "" {
		errorLog(bus.PublishChannel(), "env var DD_APP_KEY must be set for metrics to be submitted to datadog")
		return
	}

	sub, _ := bus.Subscribe("every:minute")
	defer sub.Close()

	for _ = range sub.Ch {
		processEvent(bus.PublishChannel(), localState, interestingKeys)
	}
}

func processEvent(publish chan pubsub.PubsubEvent, localState *sync.Map, interestingKeys []string) {
	for _, stateKey := range interestingKeys {
		if value, ok := localState.Load(stateKey); ok {
			value64, err := strconv.ParseFloat(value.(string), 8)
			if err == nil {
				ddSubmitGauge(publish, stateKey, value64)
			}
		} else {
			debugLog(publish, fmt.Sprintf("datadog: failed to read %s from state", stateKey))
		}
	}
}

func ddSubmitGauge(publish chan pubsub.PubsubEvent, property string, value float64) {
	ctx := datadog.NewDefaultContext(context.Background())

	nowEpoch := float64(time.Now().Unix())
	body := *datadog.NewMetricsPayload([]datadog.Series{*datadog.NewSeries(property, [][]float64{[]float64{nowEpoch, value}})})
	configuration := datadog.NewConfiguration()

	apiClient := datadog.NewAPIClient(configuration)
	_, r, err := apiClient.MetricsApi.SubmitMetrics(ctx, body)
	if err != nil {
		errorLog(publish, fmt.Sprintf("datadog: Error when calling `MetricsApi.SubmitMetrics`: %v", err))
		errorLog(publish, fmt.Sprintf("datadog: Full HTTP response: %v", r))
		return
	}

	debugLog(publish, fmt.Sprintf("datadog: Wrote MetricsApi.SubmitMetrics: %s %v", property, value))
	return
}

func debugLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "DEBUG", Value: message},
	}

}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "ERROR", Value: message},
	}

}
