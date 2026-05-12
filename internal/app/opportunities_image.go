package app

import (
	"context"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"net/http"
	"time"

	_ "golang.org/x/image/webp" // register WebP decoder
)

// logoHTTPClient is a package-level client with a short timeout — logo fetches
// shouldn't be allowed to hang the card generation indefinitely.
var logoHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// loadEmployerLogo fetches and decodes an employer logo from a URL.
// Returns nil if the URL is empty, unreachable, or undecodable — missing
// logos shouldn't block card generation.
func (app *App) loadEmployerLogo(ctx context.Context, logoURL *string) image.Image {
	if logoURL == nil || *logoURL == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *logoURL, nil)
	if err != nil {
		app.logger.Warn("building logo request", "url", *logoURL, "err", err)
		return nil
	}

	resp, err := logoHTTPClient.Do(req)
	if err != nil {
		app.logger.Warn("fetching employer logo", "url", *logoURL, "err", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.logger.Warn("logo fetch non-200", "url", *logoURL, "status", resp.StatusCode)
		return nil
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		app.logger.Warn("decoding employer logo", "url", *logoURL, "err", err)
		return nil
	}

	return img
}
