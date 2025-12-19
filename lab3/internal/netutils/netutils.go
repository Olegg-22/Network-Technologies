package netutils

import (
	"context"
	"encoding/json"
	"fmt"
	"lab3/internal/dataStruct"
	"net/http"
)

func DoGet(ctx context.Context, urlStr string, into any) error {
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
	resp, err := dataStruct.HttpClient.Do(req)
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
