// Package depscan implementa un chequeo de seguridad aislado: escanea las
// dependencias del Pull Request contra la base de datos de OSV.dev usando
// OSV-Scanner embebido como librería. A diferencia de las demás etapas, no
// pasa por ningún modelo de lenguaje — es un análisis 100% determinístico.
package depscan

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/google/osv-scanner/v2/pkg/osvscanner"

	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

var silenceLoggerOnce sync.Once

// silenceLogger evita que osv-scanner escriba logs directo a stdout/stderr,
// lo que rompería la pantalla alternativa de la TUI mientras corre el scan.
func silenceLogger() {
	silenceLoggerOnce.Do(func() {
		osvscanner.SetLogger(slog.NewTextHandler(io.Discard, nil))
	})
}

// Scan descarga los lockfiles reconocidos del commit indicado y los analiza
// con OSV-Scanner en busca de vulnerabilidades conocidas.
func Scan(pr githubpr.PullRequest, headRef string) (*review.Result, string, error) {
	if headRef == "" {
		return nil, "", fmt.Errorf(
			"no tengo el commit de referencia del pull request para escanear dependencias",
		)
	}

	silenceLogger()

	scanDir, err := os.MkdirTemp("", "code-review-depscan-*")
	if err != nil {
		return nil, "", fmt.Errorf("no pude crear un directorio temporal: %w", err)
	}
	defer os.RemoveAll(scanDir)

	downloaded, err := downloadLockfiles(pr, headRef, scanDir)
	if err != nil {
		return nil, "", err
	}

	if downloaded == 0 {
		result := &review.Result{
			Summary:  "No encontré lockfiles reconocidos en este repositorio.",
			Findings: []review.Finding{},
		}

		return result, result.Summary, nil
	}

	results, err := osvscanner.DoScan(osvscanner.ScannerActions{
		DirectoryPaths: []string{scanDir},
		Recursive:      true,
	})

	if err != nil &&
		!errors.Is(err, osvscanner.ErrVulnerabilitiesFound) &&
		!errors.Is(err, osvscanner.ErrNoPackagesFound) {

		return nil, "", fmt.Errorf("osv-scanner no pudo completar el análisis: %w", err)
	}

	result, rawOutput := buildResult(scanDir, results)

	return result, rawOutput, nil
}

// downloadLockfiles lista el árbol del repositorio en el commit indicado,
// identifica los lockfiles reconocidos (y sus manifiestos hermanos, si
// existen) y los descarga a destDir preservando su estructura relativa.
// Devuelve la cantidad de archivos descargados.
func downloadLockfiles(pr githubpr.PullRequest, ref string, destDir string) (int, error) {
	remotePaths, err := githubpr.ListTree(pr, ref)
	if err != nil {
		return 0, err
	}

	existing := make(map[string]bool, len(remotePaths))

	for _, p := range remotePaths {
		existing[p] = true
	}

	toDownload := make(map[string]bool)

	for _, remotePath := range remotePaths {
		base := path.Base(remotePath)

		if !isKnownLockfile(base) {
			continue
		}

		toDownload[remotePath] = true

		if companion := companionManifestFor(base); companion != "" {
			companionPath := path.Join(path.Dir(remotePath), companion)

			if existing[companionPath] {
				toDownload[companionPath] = true
			}
		}
	}

	for remotePath := range toDownload {
		content, err := githubpr.DownloadFile(pr, ref, remotePath)
		if err != nil {
			return 0, err
		}

		localPath := filepath.Join(destDir, filepath.FromSlash(remotePath))

		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return 0, fmt.Errorf("no pude crear %s: %w", filepath.Dir(localPath), err)
		}

		if err := os.WriteFile(localPath, content, 0o644); err != nil {
			return 0, fmt.Errorf("no pude guardar %s: %w", localPath, err)
		}
	}

	return len(toDownload), nil
}
