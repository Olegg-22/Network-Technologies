package controller

import (
	"bufio"
	"context"
	"fmt"
	"lab3/internal/dataStruct"
	"lab3/internal/location"
	"lab3/internal/utils"
	"os"
	"strconv"
	"strings"
)

func Controller() {
	keyMap, err := utils.LoadApiKeyFile(dataStruct.FileWithKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка при загрузке файла с api ключами': %v\n", err)
		os.Exit(1)
	}

	ghKey := keyMap[dataStruct.GhKey]
	owmKey := keyMap[dataStruct.OwmKey]

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Введите строку для запроса (Пример 'Цветной проезд'): ")
	q1, _ := reader.ReadString('\n')
	query := strings.TrimSpace(q1)
	if query == "" {
		fmt.Println("Пустая строка. Выход.")
		return
	}

	ctx := context.Background()
	locCh := location.SearchLocations(ctx, query, ghKey)
	res := <-locCh
	if res.Err != nil {
		fmt.Printf("Ошибка чтения локаций: %v\n", res.Err)
		return
	}

	err = utils.PrintListLocation(res)
	if err != nil {
		return
	}

	fmt.Printf("Выберите индекс локации (0..%d): ", len(res.Locs)-1)
	indexQuery, _ := reader.ReadString('\n')
	indexQuery = strings.TrimSpace(indexQuery)
	index, err := strconv.Atoi(indexQuery)
	if err != nil || index < 0 || index >= len(res.Locs) {
		fmt.Println("Неверный индекс. Выход.")
		return
	}
	chosenQuery := res.Locs[index]

	fullCh := location.FetchInfoForLocation(ctx, chosenQuery, owmKey)
	full := <-fullCh
	if full.Error != "" {
		fmt.Printf("Ошибка при поиске интересных мест поблизости: %s\n", full.Error)
	}

	utils.PrintInfoWeather(full, chosenQuery)

	utils.PrintPOIs(full)

	fmt.Println("✅ Готово.")
}
