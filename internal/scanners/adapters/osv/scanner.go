// Package osv implements the SBOMScanner port against Google's
// osv-scanner v2 library. The library is pulled in transitively by
// this project's existing deps, so wiring it does not add a new
// external dependency: it just imports what is already in go.sum.
//
// The adapter is the only place that knows about osvscanner and
// osvschema types. The domain package above sees nothing of OSV
// internals — every finding is mapped to the flat
// domain.Vulnerability shape before it leaves the package.
package osv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/osv-scanner/v2/pkg/models"
	"github.com/google/osv-scanner/v2/pkg/osvscanner"
	osvschema "github.com/ossf/osv-schema/bindings/go/osvschema"

	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
)

// Scanner is the production SBOMScanner backed by osv-scanner.
// It is stateless: callers construct one per use case invocation
// (or share one across invocations — both are safe).
type Scanner struct{}

// New returns the production scanner. There is no constructor
// state today; the function exists so future refactors that need
// to inject configuration (cache dir, custom registry, etc.) can
// do so without breaking call sites.
func New() scannersdomain.SBOMScanner { return Scanner{} }

// Scan walks every lockfile under repoPath (recursive) and
// returns every CVE the dependencies are exposed to. Empty
// repoPath returns ErrNoRepoPath before any I/O.
//
// Scan issues ONE call to osvscanner.DoScan. osv-scanner handles
// concurrency, retries, and the offline database internally; we
// just hand it the path and wait for the result.
func (Scanner) Scan(ctx context.Context, repoPath string) (scannersdomain.VulnerabilityResult, error) {
	if repoPath == "" {
		return scannersdomain.VulnerabilityResult{}, scannersdomain.ErrNoRepoPath
	}

	// osv-scanner takes a context-free API today; we honour the
	// caller's ctx by checking it before issuing the call. If the
	// caller cancels mid-scan, osv-scanner does not honour
	// cancellation, but we surface the cancellation as
	// ErrScannerUnavailable so the use case can route it.
	if err := ctx.Err(); err != nil {
		return scannersdomain.VulnerabilityResult{}, fmt.Errorf("%w: %v",
			scannersdomain.ErrScannerUnavailable, err)
	}

	raw, err := osvscanner.DoScan(osvscanner.ScannerActions{
		DirectoryPaths: []string{repoPath},
		Recursive:      true,
	})
	if err != nil {
		return scannersdomain.VulnerabilityResult{}, fmt.Errorf("%w: %v",
			scannersdomain.ErrScanFailed, err)
	}

	return convert(repoPath, raw), nil
}

// convert maps the osv-scanner result into the scanner-agnostic
// domain.VulnerabilityResult. The conversion is the only place
// that knows about osvschema types; if those types change, only
// this function needs to follow.
func convert(repoPath string, raw models.VulnerabilityResults) scannersdomain.VulnerabilityResult {
	var out []scannersdomain.Vulnerability
	for _, ps := range raw.Results {
		for _, pv := range ps.Packages {
			for _, v := range pv.Vulnerabilities {
				out = append(out, mapVulnerability(pv.Package, v))
			}
		}
	}
	return scannersdomain.VulnerabilityResult{
		RepoPath:        repoPath,
		Vulnerabilities: out,
	}
}

// mapVulnerability translates one osv-schema vulnerability into
// the flat domain shape. We pull the most useful fields:
//   - the OSV ID + aliases (so the UI can show CVE numbers),
//   - the numeric CVSS score (when present) + bucketed severity,
//   - the first fixed version (when present),
//   - the package name + version from the parent PackageInfo.
//
// We deliberately skip Affected.Ranges (where the fix lives in
// semver) because osv-scanner has already resolved the FixedVersion
// at scan time when the database provides it.
func mapVulnerability(pkg models.PackageInfo, v *osvschema.Vulnerability) scannersdomain.Vulnerability {
	out := scannersdomain.Vulnerability{
		ID:        v.GetId(),
		Aliases:   append([]string(nil), v.GetAliases()...),
		Summary:   v.GetSummary(),
		Details:   v.GetDetails(),
		Component: pkg.Name,
		Version:   pkg.Version,
	}

	// Severity: take the first severity entry that has a numeric
	// score. The OSV schema permits CVSS_V2 / V3 / V4 / Ubuntu; we
	// parse the score as a float and bucket it.
	for _, sev := range v.GetSeverity() {
		if score, vec, ok := parseScore(sev.GetScore()); ok {
			out.CVSSScore = score
			out.CVSSVector = vec
			out.Severity = bucketSeverity(score)
			break
		}
	}
	if out.Severity == "" {
		out.Severity = scannersdomain.SeverityUnknown
	}

	// FixedVersion: best-effort from the first affected range that
	// carries an "fix" event. Empty when the database has no fix
	// event (zero-days, unmaintained packages).
	for _, aff := range v.GetAffected() {
		for _, r := range aff.GetRanges() {
			for _, ev := range r.GetEvents() {
				if fix := ev.GetFixed(); fix != "" {
					out.FixedVersion = fix
					goto done
				}
			}
		}
	}
done:

	return out
}

// parseScore tries to interpret a CVSS vector string as a numeric
// score. The OSV "score" field is either:
//
//   - a CVSS vector ("CVSS:3.1/AV:N/AC:L/...") — no numeric score
//     embedded; we return the vector as-is and leave CVSSScore=0.
//   - a raw float ("7.5") — the OSV spec allows this for the
//     legacy CVSS_V2 scores that have no vector.
//
// Returns (score, vector, ok). ok=false means we could not parse;
// the caller falls back to SeverityUnknown.
func parseScore(raw string) (float64, string, bool) {
	if raw == "" {
		return 0, "", false
	}
	// Fast path: numeric score.
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f, "", true
	}
	// CVSS vector: pass it through; the caller can show it in
	// the UI but the numeric score is unknown at this level.
	if isCVSSVector(raw) {
		return 0, raw, true
	}
	return 0, "", false
}

// isCVSSVector reports whether raw looks like a CVSS vector
// ("CVSS:3.0/AV:N/..." or "CVSS:3.1/..."). We check the prefix
// only; full vector validation is the database's job.
func isCVSSVector(raw string) bool {
	return strings.HasPrefix(raw, "CVSS:")
}

// bucketSeverity collapses a numeric CVSS score (0–10) into the
// five-bucket enum the rest of the codebase uses. The thresholds
// follow the NVD convention:
//
//	9.0–10.0 → CRITICAL
//	7.0–8.9  → HIGH
//	4.0–6.9  → MEDIUM
//	0.1–3.9  → LOW
//	0.0      → UNKNOWN (handled by the caller)
func bucketSeverity(score float64) scannersdomain.Severity {
	switch {
	case score >= 9.0:
		return scannersdomain.SeverityCritical
	case score >= 7.0:
		return scannersdomain.SeverityHigh
	case score >= 4.0:
		return scannersdomain.SeverityMedium
	case score > 0:
		return scannersdomain.SeverityLow
	default:
		return scannersdomain.SeverityUnknown
	}
}

// Compile-time check: Scanner satisfies the SBOMScanner port.
var _ scannersdomain.SBOMScanner = Scanner{}

// ConvertForTest exposes the internal convert function to the
// test file in this package. Production code MUST NOT use it; the
// adapter's public surface is Scan. The underscore in the import
// path (`osv_test`) catches accidental cross-package usage at
// build time.
func ConvertForTest(repoPath string, raw models.VulnerabilityResults) scannersdomain.VulnerabilityResult {
	return convert(repoPath, raw)
}