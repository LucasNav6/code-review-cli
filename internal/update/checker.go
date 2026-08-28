package update

import (
	"github.com/LucasNav6/code-review-cli/internal/buildinfo"

	"golang.org/x/mod/semver"
)

// CheckResult resume la comparación entre la versión instalada y la
// última publicada.
type CheckResult struct {
	Current    string
	Latest     string
	HasUpdate  bool
	ReleaseURL string
}

// Check compara la versión actual contra la última release del repositorio
// configurado en buildinfo.Repo. Usa una caché en disco para no consultar
// GitHub en cada ejecución. Si no hay repositorio configurado o el binario
// es un build de desarrollo, no hace ninguna llamada de red.
func Check() (*CheckResult, error) {
	return checkWith(buildinfo.Repo, buildinfo.Version, LatestRelease, readCache, writeCache)
}

type latestReleaseFunc func(repo string) (*Release, error)
type readCacheFunc func() (*cacheEntry, bool)
type writeCacheFunc func(latestVersion string)

func checkWith(
	repo string,
	currentVersion string,
	fetchLatest latestReleaseFunc,
	loadCache readCacheFunc,
	saveCache writeCacheFunc,
) (*CheckResult, error) {
	if repo == "" {
		return nil, nil
	}

	if !semver.IsValid(currentVersion) {
		// Build local ("dev") u otro esquema de versionado: no hay contra
		// qué comparar de forma confiable.
		return nil, nil
	}

	latest := ""

	if entry, ok := loadCache(); ok {
		latest = entry.LatestVersion
	} else {
		release, err := fetchLatest(repo)
		if err != nil {
			return nil, err
		}

		latest = release.TagName
		saveCache(latest)
	}

	if !semver.IsValid(latest) {
		return nil, nil
	}

	result := &CheckResult{
		Current:    currentVersion,
		Latest:     latest,
		HasUpdate:  semver.Compare(latest, currentVersion) > 0,
		ReleaseURL: "https://github.com/" + repo + "/releases/tag/" + latest,
	}

	return result, nil
}
