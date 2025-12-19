package location

import (
	"context"
	"fmt"
	"lab3/internal/POIs"
	"lab3/internal/dataStruct"
	"lab3/internal/infoLocation"
	"lab3/internal/netutils"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

func FetchInfoForLocation(parentCtx context.Context, loc dataStruct.Location, owmKey string) <-chan dataStruct.FullResult {
	out := make(chan dataStruct.FullResult, 1)
	go func() {
		defer close(out)
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
		defer cancel()

		var (
			weather    *dataStruct.OWMResp
			resultPOIs []map[string]any
		)

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			u := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&appid=%s&units=metric&lang=ru", loc.Lat, loc.Lon, owmKey)
			var ow dataStruct.OWMResp
			if err := netutils.DoGet(ctx, u, &ow); err != nil {
				return fmt.Errorf("openweather: %w", err)
			}
			weather = &ow
			return nil
		})

		g.Go(func() error {
			pois, err := POIs.SearchPOIsOverpass(ctx, loc.Lat, loc.Lon)
			if err != nil {
				return err
			}

			if len(pois) > 5 {
				pois = pois[:5]
			}

			sem := make(chan struct{}, 5)
			g2, ctx2 := errgroup.WithContext(ctx)

			for i := range pois {
				i := i
				nameIfc, _ := pois[i]["name"]
				name, _ := nameIfc.(string)
				if name == "" {
					continue
				}

				wpIfc, _ := pois[i]["wikipedia"]
				if wpS, ok := wpIfc.(string); ok && wpS != "" {
					parts := strings.SplitN(wpS, ":", 2)
					if len(parts) == 2 {
						name = parts[1]
					}
				}

				sem <- struct{}{}
				g2.Go(func() error {
					defer func() { <-sem }()
					desc, err := infoLocation.FetchWikiSummary(ctx2, name)
					if err != nil {
						pois[i]["description"] = ""
						return nil
					}
					pois[i]["description"] = desc
					return nil
				})
			}

			_ = g2.Wait()
			resultPOIs = pois
			return nil
		})

		if err := g.Wait(); err != nil {
			out <- dataStruct.FullResult{Location: loc, Error: err.Error()}
			return
		}

		out <- dataStruct.FullResult{Location: loc, Weather: weather, POIs: resultPOIs}
	}()
	return out
}
