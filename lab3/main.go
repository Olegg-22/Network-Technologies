package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

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

type FullResult struct {
	Location Location         `json:"location"`
	Weather  *OWMResp         `json:"weather,omitempty"`
	POIs     []map[string]any `json:"pois,omitempty"`
	Error    string           `json:"error,omitempty"`
}

func loadFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	keyMap := make(map[string]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, "\"'")
		if k != "" && v != "" {
			keyMap[k] = v
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return keyMap, nil
}

func doGet(ctx context.Context, urlStr string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "olegg")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d from %s", resp.StatusCode, urlStr)
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(into)
}

type LocsResult struct {
	Locs []Location
	Err  error
}

func SearchLocations(ctx context.Context, q, ghKey string) <-chan LocsResult {
	out := make(chan LocsResult, 1)
	go func() {
		defer close(out)
		u := fmt.Sprintf("https://graphhopper.com/api/1/geocode?q=%s&limit=10&key=%s", url.QueryEscape(q), ghKey)
		var gh GHGeocodeResp
		if err := doGet(ctx, u, &gh); err != nil {
			out <- LocsResult{Err: fmt.Errorf("graphhopper geocode: %w", err)}
			return
		}
		locs := make([]Location, 0, len(gh.Hits))
		for _, h := range gh.Hits {
			placeType := fmt.Sprintf("%s, %s", h.OsmKey, h.OsmValue)
			locs = append(locs, Location{
				Name:      h.Name,
				Country:   h.Country,
				City:      h.City,
				Lat:       h.Point.Lat,
				Lon:       h.Point.Lng,
				PlaceType: placeType,
			})
		}
		out <- LocsResult{Locs: locs}
	}()
	return out
}

func SearchPOIsOverpass(ctx context.Context, lat, lon float64) ([]map[string]any, error) {
	//way(around:1000,%.6f,%.6f)["tourism"];way(around:1000,%.6f,%.6f)["historic"];relation(around:1000,%.6f,%.6f)["tourism"];relation(around:1000,%.6f,%.6f)["historic"];
	query := fmt.Sprintf(`[out:json];(node(around:1000,%.6f,%.6f)["shop"];node(around:1000,%.6f,%.6f)["amenity"];);out;`, lat, lon, lat, lon)
	urlStr := "https://overpass-api.de/api/interpreter?data=" + url.QueryEscape(query)

	var resp OverpassResp
	if err := doGet(ctx, urlStr, &resp); err != nil {
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

type WikiSummary struct {
	Title   string `json:"title"`
	Extract string `json:"extract"`
}

func FetchWikiSummary(ctx context.Context, title string) (string, error) {
	title = strings.ReplaceAll(title, " ", "_")
	urlStr := fmt.Sprintf("https://ru.wikipedia.org/api/rest_v1/page/summary/%s", url.QueryEscape(title))

	var ws WikiSummary
	if err := doGet(ctx, urlStr, &ws); err != nil {
		return "", fmt.Errorf("wikipedia: %w", err)
	}
	return ws.Extract, nil
}

func FetchInfoForLocation(parentCtx context.Context, loc Location, owmKey string) <-chan FullResult {
	out := make(chan FullResult, 1)
	go func() {
		defer close(out)
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
		defer cancel()

		var (
			weather    *OWMResp
			resultPOIs []map[string]any
		)

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			u := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&appid=%s&units=metric&lang=ru", loc.Lat, loc.Lon, owmKey)
			var ow OWMResp
			if err := doGet(ctx, u, &ow); err != nil {
				return fmt.Errorf("openweather: %w", err)
			}
			weather = &ow
			return nil
		})

		g.Go(func() error {
			pois, err := SearchPOIsOverpass(ctx, loc.Lat, loc.Lon)
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
					desc, err := FetchWikiSummary(ctx2, name)
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
			out <- FullResult{Location: loc, Error: err.Error()}
			return
		}

		out <- FullResult{Location: loc, Weather: weather, POIs: resultPOIs}
	}()
	return out
}

func main() {
	keyMap, err := loadFile("./.data/apiKey")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка при загрузке data файла './.data/apiKey': %v\n", err)
		os.Exit(1)
	}

	ghKey := keyMap["GH_API_KEY"]
	owmKey := keyMap["OWM_API_KEY"]

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Введите строку для запроса (Пример 'Цветной проезд'): ")
	q1, _ := reader.ReadString('\n')
	query := strings.TrimSpace(q1)
	if query == "" {
		fmt.Println("Пустая строка. Выход.")
		return
	}

	ctx := context.Background()
	locCh := SearchLocations(ctx, query, ghKey)
	res := <-locCh
	if res.Err != nil {
		fmt.Printf("Ошибка чтения локаций: %v\n", res.Err)
		return
	}
	if len(res.Locs) == 0 {
		fmt.Println("Не нашлось локаций.")
		return
	}
	fmt.Println("Найденные локации:")
	for i, l := range res.Locs {
		extra := l.Country
		if l.City != "" {
			extra = l.City + ", " + l.Country
		}
		typ := ""
		if l.PlaceType != "" {
			typ = " — " + l.PlaceType
		}
		fmt.Printf("[%d] %s (%s%s) (lat: %.6f lon: %.6f)\n", i, l.Name, extra, typ, l.Lat, l.Lon)
	}
	fmt.Printf("Choose index of location (0..%d): ", len(res.Locs)-1)
	indexQuery, _ := reader.ReadString('\n')
	indexQuery = strings.TrimSpace(indexQuery)
	index, err := strconv.Atoi(indexQuery)
	if err != nil || index < 0 || index >= len(res.Locs) {
		fmt.Println("Неверный индекс. Выход.")
		return
	}
	chosenQuery := res.Locs[index]

	fullCh := FetchInfoForLocation(ctx, chosenQuery, owmKey)
	full := <-fullCh
	if full.Error != "" {
		fmt.Printf("Ошибка при поиске интересных мест поблизости: %s\n", full.Error)
	}
	fmt.Printf("\nИнформация о месте: %s (%s)\n", chosenQuery.Name, chosenQuery.Country)
	if full.Weather != nil {
		fmt.Printf("🌡️  Температура: %.1f°C\n", full.Weather.Main.Temp)
		fmt.Printf("🌥️  Погода: %s\n", full.Weather.Weather[0].Description)
		pressureHpa := float64(full.Weather.Main.Pressure)
		pressureMmHg := pressureHpa * 0.750062
		fmt.Printf("🧭  Давление: %.1f мм рт. ст.\n", pressureMmHg)
	}

	fmt.Println("\n🏛️  Интересные места поблизости:")
	if len(full.POIs) == 0 {
		fmt.Println("  (ничего не найдено)")
	} else {
		for _, p := range full.POIs {
			fmt.Printf("— %s\n", p["name"])
			desc, ok := p["description"].(string)
			if ok && desc != "" {
				fmt.Printf("  %s\n\n", desc)
			} else {
				fmt.Printf("Описания не найдено\n")
			}
		}
	}
	fmt.Println("✅ Готово.")
}
