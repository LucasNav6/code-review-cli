// Package domain owns the pure types and contracts of the scanners
// bounded context. It is the source of truth for:
//
//   - Vulnerability: one CVE/vulnerability finding from any scanner.
//   - SBOMScanner: the contract every scanner backend satisfies.
//   - VulnerabilityResult: the aggregate returned by a scan.
//
// Today only OSVScanner (the osv-scanner adapter) implements the
// contract. Tomorrow SecretScanner (gitleaks, trufflehog) and
// QualityGateScanner (sonarqube, datadog) plug in as additional
// adapters without changing this package.
package domain

import "errors"

// Sentinel errors raised inside the scanners bounded context. They
// are declared here (not in helpers/) because they are part of the
// contract a future caller can rely on without depending on the
// helpers package.
var (
	// ErrScannerUnavailable is returned when the scanner backend
	// cannot be reached (network down, registry unreachable, etc.).
	ErrScannerUnavailable = errors.New("scanners: backend unavailable")

	// ErrScanFailed is the catch-all for a scan that started but
	// could not produce a result. The original error is preserved
	// via wrapping so the caller can surface actionable messages.
	ErrScanFailed = errors.New("scanners: scan failed")

	// ErrNoRepoPath is returned when the caller hands the scanner
	// an empty repository path. The scan needs filesystem access
	// to walk lockfiles (go.mod, package-lock.json, etc.).
	ErrNoRepoPath = errors.New("scanners: repo path is required")

	// ErrNoVulnerabilities is NOT an error — it is the success
	// signal for "scanned, found nothing". Exposed so the use case
	// can distinguish "scan ran clean" from "scan was skipped".
	// (Implemented by returning an empty slice, not by error.)
)