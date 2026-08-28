package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Release es la porción de la respuesta de la API de Releases de GitHub
// que nos interesa.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset es un artefacto adjunto a una release (un binario para una
// plataforma puntual, o el archivo de checksums).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

const requestTimeout = 6 * time.Second

// LatestRelease consulta la última release publicada de un repositorio
// "owner/repo" en GitHub.
func LatestRelease(repo string) (*Release, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "code-review-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no pude consultar GitHub Releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("el repositorio %q todavía no tiene releases publicadas", repo)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("GitHub respondió %d: %s", resp.StatusCode, string(body))
	}

	var release Release

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("no pude interpretar la respuesta de GitHub: %w", err)
	}

	return &release, nil
}

// Download descarga un asset a un io.Writer.
func Download(ctx context.Context, downloadURL string, dst io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "code-review-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("no pude descargar la actualización: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("la descarga respondió %d", resp.StatusCode)
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("no pude guardar la descarga: %w", err)
	}

	return nil
}
