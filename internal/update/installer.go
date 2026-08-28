package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"

	"golang.org/x/mod/semver"
)

// ErrNotConfigured se devuelve cuando no hay un repositorio de releases
// configurado en el binario (buildinfo.Repo vacío).
var ErrNotConfigured = fmt.Errorf(
	"esta build de code-review todavía no tiene un repositorio de releases configurado, así que no puedo autoactualizarme. Descargá la última versión manualmente",
)

// ErrUpToDate se devuelve cuando ya se tiene la última versión disponible.
var ErrUpToDate = fmt.Errorf("ya estás en la última versión")

const binaryName = "code-review"

// UpgradeResult resume el resultado de un `code-review upgrade` exitoso.
type UpgradeResult struct {
	PreviousVersion string
	NewVersion      string
	InstalledPath   string
}

// Upgrade descarga la última release para la plataforma actual y reemplaza
// el binario en ejecución. Devuelve ErrUpToDate si ya es la última versión
// y ErrNotConfigured si el binario no tiene repositorio de releases.
func Upgrade(ctx context.Context) (*UpgradeResult, error) {
	if !buildinfo.UpdatesConfigured() {
		return nil, ErrNotConfigured
	}

	release, err := LatestRelease(buildinfo.Repo)
	if err != nil {
		return nil, err
	}

	if semver.IsValid(buildinfo.Version) &&
		semver.IsValid(release.TagName) &&
		semver.Compare(release.TagName, buildinfo.Version) <= 0 {

		return nil, ErrUpToDate
	}

	asset, err := findAsset(release.Assets)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	if err := Download(ctx, asset.BrowserDownloadURL, &buf); err != nil {
		return nil, err
	}

	binary, err := extractBinary(asset.Name, buf.Bytes())
	if err != nil {
		return nil, err
	}

	installedPath, err := replaceExecutable(binary)
	if err != nil {
		return nil, err
	}

	return &UpgradeResult{
		PreviousVersion: buildinfo.Version,
		NewVersion:      release.TagName,
		InstalledPath:   installedPath,
	}, nil
}

func assetName() string {
	ext := "tar.gz"

	if runtime.GOOS == "windows" {
		ext = "zip"
	}

	return fmt.Sprintf("%s_%s_%s.%s", binaryName, runtime.GOOS, runtime.GOARCH, ext)
}

func findAsset(assets []Asset) (*Asset, error) {
	want := assetName()

	for _, asset := range assets {
		if asset.Name == want {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf(
		"la última release no tiene un binario para %s/%s (esperaba %q)",
		runtime.GOOS,
		runtime.GOARCH,
		want,
	)
}

func extractBinary(archiveName string, data []byte) ([]byte, error) {
	wantFile := binaryName
	if runtime.GOOS == "windows" {
		wantFile += ".exe"
	}

	if filepath.Ext(archiveName) == ".zip" {
		return extractFromZip(data, wantFile)
	}

	return extractFromTarGz(data, wantFile)
}

func extractFromTarGz(data []byte, wantFile string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("el archivo descargado no es un .tar.gz válido: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("no pude leer el archivo descargado: %w", err)
		}

		if filepath.Base(header.Name) != wantFile {
			continue
		}

		return io.ReadAll(tr)
	}

	return nil, fmt.Errorf("no encontré %q dentro del archivo descargado", wantFile)
}

func extractFromZip(data []byte, wantFile string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("el archivo descargado no es un .zip válido: %w", err)
	}

	for _, file := range zr.File {
		if filepath.Base(file.Name) != wantFile {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		return io.ReadAll(rc)
	}

	return nil, fmt.Errorf("no encontré %q dentro del archivo descargado", wantFile)
}

// replaceExecutable escribe el nuevo binario y reemplaza el ejecutable
// actual de forma atómica (o lo más atómica posible según el sistema
// operativo).
func replaceExecutable(binary []byte) (string, error) {
	currentPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("no pude determinar la ubicación del binario actual: %w", err)
	}

	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		return "", fmt.Errorf("no pude resolver la ubicación del binario actual: %w", err)
	}

	dir := filepath.Dir(currentPath)

	tmpFile, err := os.CreateTemp(dir, ".code-review-update-*")
	if err != nil {
		return "", fmt.Errorf("no tengo permisos para escribir en %s: %w", dir, err)
	}

	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(binary); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("no pude escribir el nuevo binario: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", fmt.Errorf("no pude marcar el nuevo binario como ejecutable: %w", err)
	}

	if runtime.GOOS == "windows" {
		// En Windows no se puede sobrescribir un ejecutable en uso: hay que
		// sacarlo del camino primero.
		oldPath := currentPath + ".old"
		_ = os.Remove(oldPath)

		if err := os.Rename(currentPath, oldPath); err != nil {
			return "", fmt.Errorf("no pude mover el binario actual: %w", err)
		}
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		return "", fmt.Errorf("no pude instalar el nuevo binario en %s: %w", currentPath, err)
	}

	return currentPath, nil
}
