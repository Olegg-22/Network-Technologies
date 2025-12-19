package utils

import (
	"bufio"
	"fmt"
	"lab3/internal/dataStruct"
	"os"
	"strings"
)

func PrintListLocation(result dataStruct.LocsResult) error {
	if len(result.Locs) == 0 {
		fmt.Println("Не нашлось локаций.")
		return fmt.Errorf("no locations found")
	}
	fmt.Println("Найденные локации:")
	for i, l := range result.Locs {
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
	return nil
}

func PrintInfoWeather(full dataStruct.FullResult, location dataStruct.Location) {
	fmt.Printf("\nИнформация о месте: %s (%s)\n", location.Name, location.Country)
	if full.Weather != nil {
		fmt.Printf("🌡️  Температура: %.1f°C\n", full.Weather.Main.Temp)
		fmt.Printf("🌥️  Погода: %s\n", full.Weather.Weather[0].Description)
		pressureHpa := float64(full.Weather.Main.Pressure)
		pressureMmHg := pressureHpa * 0.750062
		fmt.Printf("🧭  Давление: %.1f мм рт. ст.\n", pressureMmHg)
	}
}
func PrintPOIs(full dataStruct.FullResult) {
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
}

func LoadApiKeyFile(path string) (map[string]string, error) {
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
