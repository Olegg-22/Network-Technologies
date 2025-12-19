package infoLocation

import (
	"context"
	"fmt"
	"lab3/internal/netutils"
	"net/url"
	"strings"
)

type WikiSummary struct {
	Title   string `json:"title"`
	Extract string `json:"extract"`
}

func FetchWikiSummary(ctx context.Context, title string) (string, error) {
	title = strings.ReplaceAll(title, " ", "_")
	urlStr := fmt.Sprintf("https://ru.wikipedia.org/api/rest_v1/page/summary/%s", url.QueryEscape(title))

	var ws WikiSummary
	if err := netutils.DoGet(ctx, urlStr, &ws); err != nil {
		return "", fmt.Errorf("wikipedia: %w", err)
	}
	return ws.Extract, nil
}
