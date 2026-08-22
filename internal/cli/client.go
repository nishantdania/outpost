package cli

import (
	"context"
	"github.com/nishantdania/outpost/internal/config"
	"net/http"
	"strings"
	"time"
)

func daemonURL() (string, error) {
	cfg, err := config.LoadClient()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(cfg.DaemonURL, "/"), nil
}
func request(ctx context.Context, method, path string) (*http.Response, error) {
	base, err := daemonURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, nil)
	if err != nil {
		return nil, err
	}
	return (&http.Client{Timeout: 30 * time.Second}).Do(req)
}
