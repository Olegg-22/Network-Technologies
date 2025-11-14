package dataStruct

import (
	"net/http"
	"time"
)

const (
	GhKey            = "GH_API_KEY"
	OwmKey           = "OWM_API_KEY"
	FileWithKey      = ".data/apiKey"
	LimitForLocation = 10
)

var (
	HttpClient = &http.Client{Timeout: 15 * time.Second}
)

type Location struct {
	Name      string  `json:"name"`
	Country   string  `json:"country"`
	City      string  `json:"city,omitempty"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	PlaceType string  `json:"place_type,omitempty"`
}

type OWMResp struct {
	Weather []struct {
		Main        string `json:"main"`
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp     float64 `json:"temp"`
		Pressure int     `json:"pressure"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Name string `json:"name"`
}

type FullResult struct {
	Location Location         `json:"location"`
	Weather  *OWMResp         `json:"weather,omitempty"`
	POIs     []map[string]any `json:"pois,omitempty"`
	Error    string           `json:"error,omitempty"`
}

type LocsResult struct {
	Locs []Location
	Err  error
}
