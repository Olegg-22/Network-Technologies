package location

import (
	"context"
	"fmt"
	"lab3/internal/dataStruct"
	"lab3/internal/netutils"
	"net/url"
)

type GHGeocodeResp struct {
	Hits []struct {
		Name     string `json:"name"`
		Country  string `json:"country"`
		City     string `json:"city"`
		Street   string `json:"street"`
		OsmKey   string `json:"osm_key"`
		OsmValue string `json:"osm_value"`
		Point    struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"point"`
	} `json:"hits"`
}

func SearchLocations(ctx context.Context, q, ghKey string) <-chan dataStruct.LocsResult {
	out := make(chan dataStruct.LocsResult, 1)
	go func() {
		defer close(out)
		u := fmt.Sprintf("https://graphhopper.com/api/1/geocode?q=%s&limit=%d&key=%s", url.QueryEscape(q), dataStruct.LimitForLocation, ghKey)
		var gh GHGeocodeResp
		if err := netutils.DoGet(ctx, u, &gh); err != nil {
			out <- dataStruct.LocsResult{Err: fmt.Errorf("graphhopper geocode: %w", err)}
			return
		}
		locs := make([]dataStruct.Location, 0, len(gh.Hits))
		for _, h := range gh.Hits {
			placeType := fmt.Sprintf("%s, %s", h.OsmKey, h.OsmValue)
			locs = append(locs, dataStruct.Location{
				Name:      h.Name,
				Country:   h.Country,
				City:      h.City,
				Lat:       h.Point.Lat,
				Lon:       h.Point.Lng,
				PlaceType: placeType,
			})
		}
		out <- dataStruct.LocsResult{Locs: locs}
	}()
	return out
}
