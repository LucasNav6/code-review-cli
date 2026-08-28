//go:build smoke

// Test de integración opcional: pega contra la API real de OSV.dev, por eso
// queda afuera del suite normal.
//
//	go test -tags smoke ./internal/depscan/ -run TestRealScanFindsKnownVulnerability -v
package depscan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/osv-scanner/v2/pkg/osvscanner"
)

func TestRealScanFindsKnownVulnerability(t *testing.T) {
	scanDir := t.TempDir()

	// lodash@4.17.15 tiene varias CVEs públicas conocidas (Command
	// Injection, Prototype Pollution, ReDoS) — sirve como fixture estable
	// para validar que la integración con OSV.dev funciona de punta a punta.
	lockfile := `{
		"name": "smoke-test",
		"version": "1.0.0",
		"lockfileVersion": 1,
		"requires": true,
		"dependencies": {
			"lodash": {
				"version": "4.17.15",
				"resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.15.tgz"
			}
		}
	}`

	path := filepath.Join(scanDir, "package-lock.json")

	if err := os.WriteFile(path, []byte(lockfile), 0o644); err != nil {
		t.Fatalf("no pude escribir el fixture: %v", err)
	}

	silenceLogger()

	results, err := osvscanner.DoScan(osvscanner.ScannerActions{
		DirectoryPaths: []string{scanDir},
		Recursive:      true,
	})

	if err != nil && err != osvscanner.ErrVulnerabilitiesFound {
		t.Fatalf("DoScan falló: %v", err)
	}

	result, raw := buildResult(scanDir, results)

	if len(result.Findings) == 0 {
		t.Fatalf("esperaba al menos una vulnerabilidad conocida para lodash@4.17.15, salida cruda:\n%s", raw)
	}

	for _, finding := range result.Findings {
		if finding.File != "package-lock.json" {
			t.Errorf("finding.File = %q, esperaba \"package-lock.json\"", finding.File)
		}

		if finding.Category == "" {
			t.Errorf("finding sin categoría de severidad: %+v", finding)
		}
	}
}
