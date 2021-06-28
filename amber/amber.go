package amber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	BaseURLV1 = "https://api.amber.com.au/v1"
)

type Client struct {
	BaseURL    string
	apiKey     string
	HTTPClient *http.Client
}

type errorResponse struct {
	Message string `json:"message"`
}

type Site struct {
	Id  string `json:"id"`
	Nmi string `json:"nmi"`
}

type Price struct {
	Type        string  `json:"type"`
	Date        string  `json:"date"`
	Duration    int     `json:"duration"`
	StartTime   string  `json:"startTime"`
	EndTime     string  `json:"endTime"`
	NemTime     string  `json:"nemTime"`
	PerKwh      float64 `json:"perKwh"`
	Renewables  float64 `json:"renewables"`
	SpotPerKwh  float64 `json:"spotPerKwh"`
	ChannelType string  `json:"channelType"`
	SpikeStatus string  `json:"spikeStatus"`
	Estimate    bool    `json:"estimate"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL: BaseURLV1,
		apiKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func (c *Client) GetSites(ctx context.Context) ([]Site, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/sites", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	res := []Site{}
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) GetCurrentPrices(ctx context.Context, site Site) ([]Price, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/sites/%s/prices/current?resolution=30", c.BaseURL, site.Id), nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	res := []Price{}
	if err := c.sendRequest(req, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) sendRequest(req *http.Request, v interface{}) error {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		var errRes errorResponse
		if err = json.NewDecoder(res.Body).Decode(&errRes); err == nil {
			return errors.New(errRes.Message)
		}

		return fmt.Errorf("unknown error, status code: %d", res.StatusCode)
	}

	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		return err
	}

	return nil
}
