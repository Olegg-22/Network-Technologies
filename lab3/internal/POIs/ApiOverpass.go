package POIs

import (
	"context"
	"fmt"
	"lab3/internal/netutils"
	"net/url"
)

type OverpassResp struct {
	Elements []struct {
		Type string  `json:"type"`
		ID   int64   `json:"id"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Tags struct {
			Name      string `json:"name"`
			Tourism   string `json:"tourism"`
			Historic  string `json:"historic"`
			Wikipedia string `json:"wikipedia"`
		} `json:"tags"`
	} `json:"elements"`
}

func SearchPOIsOverpass(ctx context.Context, lat, lon float64) ([]map[string]any, error) {
	//way(around:1000,%.6f,%.6f)["tourism"];way(around:1000,%.6f,%.6f)["historic"];relation(around:1000,%.6f,%.6f)["tourism"];relation(around:1000,%.6f,%.6f)["historic"];
	query := fmt.Sprintf(`[out:json];(node(around:1000,%.6f,%.6f)["shop"];node(around:1000,%.6f,%.6f)["amenity"];);out;`, lat, lon, lat, lon)
	urlStr := "https://overpass-api.de/api/interpreter?data=" + url.QueryEscape(query)

	var resp OverpassResp
	if err := netutils.DoGet(ctx, urlStr, &resp); err != nil {
		return nil, fmt.Errorf("overpass: %w", err)
	}

	pois := make([]map[string]any, 0, len(resp.Elements))
	for _, e := range resp.Elements {
		name := e.Tags.Name
		if name == "" {
			continue
		}
		pois = append(pois, map[string]any{
			"name":      name,
			"tourism":   e.Tags.Tourism,
			"historic":  e.Tags.Historic,
			"wikipedia": e.Tags.Wikipedia,
			"lat":       e.Lat,
			"lon":       e.Lon,
		})
	}
	return pois, nil
}
