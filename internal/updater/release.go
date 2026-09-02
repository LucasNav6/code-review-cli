// Package updater implementa la lógica real de upgrade/downgrade/
// uninstall contra los binarios publicados en GitHub Releases.
//
// No usa librerías externas: el listado de releases se descarga con
// net/http y la comparación de versiones se hace con golang.org/x/mod/
// semver (que ya está en go.mod).
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/LucasNav6/code-review-cli/buildinfo"
)

// releaseListURL devuelve la URL de la API de GitHub Releases para el
// repo configurado en buildinfo.Repo. La lista está ordenada por fecha
// de creación descendente (la primera es la latest non-prerelease).
func releaseListURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases", buildinfo.Repo)
}

// Release es la subestructura del JSON de GitHub que nos interesa.
// Sólo parseamos lo necesario para comparar versiones y bajar el asset.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

// Asset representa un archivo adjunto a una release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// LatestStable descarga todas las releases del repo, descarta
// drafts y prereleases, y devuelve la más alta según semver.
//
// Devuelve un error con mensaje accionable si no se puede contactar
// GitHub, si la respuesta no es JSON válido, o si el repo no tiene
// ninguna release estable.
func LatestStable(ctx context.Context) (*Release, error) {
	releases, err := listReleases(ctx)
	if err != nil {
		return nil, err
	}

	var candidates []*Release
	for i := range releases {
		r := &releases[i]
		if r.Draft || r.Prerelease {
			continue
		}
		// semver exige prefijo "v"; los tags de GitHub suelen traerlo.
		tag := normaliseTag(r.TagName)
		if !semver.IsValid(tag) {
			// Tags que no son semver se ignoran silenciosamente.
			// Es lo que hace `gh release list --exclude-pre-releases`.
			continue
		}
		candidates = append(candidates, r)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no stable releases found for %s", buildinfo.Repo)
	}

	// GitHub ya ordena por fecha desc, pero queremos la mayor por semver
	// para no atarnos a la fecha de publicación.
	best := candidates[0]
	bestTag := normaliseTag(best.TagName)
	for _, r := range candidates[1:] {
		rt := normaliseTag(r.TagName)
		if semver.Compare(rt, bestTag) > 0 {
			best = r
			bestTag = rt
		}
	}
	return best, nil
}

// FindRelease busca una release por tag exacto (ej: "v1.2.3" o "1.2.3").
// Acepta tags con o sin prefijo "v". Devuelve nil si no existe.
func FindRelease(ctx context.Context, tag string) (*Release, error) {
	releases, err := listReleases(ctx)
	if err != nil {
		return nil, err
	}

	want := normaliseTag(tag)
	for i := range releases {
		if normaliseTag(releases[i].TagName) == want {
			return &releases[i], nil
		}
	}
	return nil, nil
}

// listReleases descarga la lista de releases (hasta 100, que es el
// máximo que devuelve la API sin paginar). 100 nos alcanza para
// cualquier proyecto serio.
func listReleases(ctx context.Context) ([]Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseListURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("could not build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach GitHub Releases: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read GitHub response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Releases responded %s: %s",
			resp.Status, truncate(string(body), 200))
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("could not parse GitHub Releases JSON: %w", err)
	}
	return releases, nil
}

// AssetForPlatform devuelve el asset de la release correspondiente a
// la plataforma actual (runtime.GOOS + runtime.GOARCH). El nombre
// sigue la convención de goreleaser:
//
//	code-review_<goos>_<goarch>.tar.gz
//
// Devuelve nil si la release no trae un asset para esta plataforma
// (lo que normalmente indica que el mantenedor todavía no la publicó).
func (r *Release) AssetForPlatform() *Asset {
	want := assetName(runtime.GOOS, runtime.GOARCH)
	for i := range r.Assets {
		if r.Assets[i].Name == want {
			return &r.Assets[i]
		}
	}
	return nil
}

// DownloadURL devuelve la URL desde la que bajar el binario para la
// plataforma actual. Es una convenience que combina FindRelease/
// LatestStable con AssetForPlatform y falla con un mensaje claro si
// falta el asset para esta plataforma.
func (r *Release) DownloadURL() (string, error) {
	a := r.AssetForPlatform()
	if a == nil {
		return "", fmt.Errorf("release %s has no asset for %s/%s (expected %s)",
			r.TagName, runtime.GOOS, runtime.GOARCH, assetName(runtime.GOOS, runtime.GOARCH))
	}
	return a.BrowserDownloadURL, nil
}

// IsNewer devuelve true si el tag de la release es estrictamente
// mayor (en semver) que la versión pasada. Acepta "v1.2.3" o
// "1.2.3" como current. current vacío o inválido se considera
// "cualquier versión más alta es nueva".
func (r *Release) IsNewer(current string) bool {
	cur := normaliseTag(current)
	if !semver.IsValid(cur) {
		return true // build dev, o versión desconocida -> siempre "hay upgrade"
	}
	return semver.Compare(normaliseTag(r.TagName), cur) > 0
}

// IsOlder devuelve true si el tag de la release es estrictamente
// menor (en semver) que current. Misma convención que IsNewer.
func (r *Release) IsOlder(current string) bool {
	cur := normaliseTag(current)
	if !semver.IsValid(cur) {
		return false // sin versión conocida no se puede hablar de "menor"
	}
	return semver.Compare(normaliseTag(r.TagName), cur) < 0
}

// assetName reproduce el template del .goreleaser.yaml:
//
//	name_template: "code-review_{{ .Os }}_{{ .Arch }}"
//
// Se mantiene en sync manualmente porque no podemos invocar goreleaser
// desde runtime. Si cambiás el template, cambiá esto también.
func assetName(goos, goarch string) string {
	if goos == "windows" {
		return fmt.Sprintf("code-review_%s_%s.zip", goos, goarch)
	}
	return fmt.Sprintf("code-review_%s_%s.tar.gz", goos, goarch)
}

// normaliseTag garantiza que el tag tenga prefijo "v" (lo que exige
// golang.org/x/mod/semver). Devuelve el string vacío si la entrada
// está vacía.
func normaliseTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	if !strings.HasPrefix(tag, "v") {
		// Limpia prefijos comunes tipo "release-1.2.3" -> "v1.2.3"
		// si vienen con el formato estándar.
		if i := strings.LastIndex(tag, "-"); i >= 0 {
			suffix := strings.TrimPrefix(tag[i:], "-")
			if semver.IsValid("v" + suffix) {
				return "v" + suffix
			}
		}
		return "v" + tag
	}
	return tag
}

// userAgent es lo que GitHub ve en el header. Lo construimos con la
// versión del binario para que el maintainer pueda filtrar tráfico
// desde el panel de GitHub si hace falta.
func userAgent() string {
	v := buildinfo.Version
	if v == "" {
		v = "unknown"
	}
	return fmt.Sprintf("code-review-cli/%s (+https://github.com/%s)", v, buildinfo.Repo)
}

// truncate acorta un string a n bytes para mensajes de error
// (las respuestas de GitHub pueden ser largas).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// HostOf es una helper expuesta por si los comandos necesitan saber
// la URL canónica del repo (por ejemplo para imprimirla en un hint).
// Devuelve "" si el repo está malformado.
func HostOf() string {
	if buildinfo.Repo == "" {
		return ""
	}
	u := "https://github.com/" + buildinfo.Repo
	if _, err := url.Parse(u); err != nil {
		return ""
	}
	return u
}
