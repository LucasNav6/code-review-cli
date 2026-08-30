// Package secretscan implementa un chequeo de seguridad aislado: clona el
// repositorio del Pull Request y escanea el rango de commits que introduce
// con gitleaks, en busca de secretos expuestos. Igual que internal/depscan,
// es 100% determinístico: no pasa por ningún modelo de lenguaje.
package secretscan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

// ErrGitleaksNotInstalled se devuelve cuando el binario "gitleaks" no está
// disponible. A diferencia de gh/claude, gitleaks no es una dependencia dura
// del resto del pipeline: si falta, la etapa directamente no se corre (ver
// Available).
var ErrGitleaksNotInstalled = fmt.Errorf(
	"no encontré el comando \"gitleaks\". Instalalo desde https://github.com/gitleaks/gitleaks",
)

// Available indica si el binario "gitleaks" está instalado. gitleaks es
// opcional: si no está, esta etapa no se programa (no aparece como error,
// simplemente no corre), a diferencia de gh/claude que sí son obligatorios.
func Available() bool {
	_, err := exec.LookPath("gitleaks")

	return err == nil
}

// Scan clona el repositorio del Pull Request en un directorio temporal y
// corre gitleaks sobre los commits introducidos entre la rama base y el
// commit de referencia del PR.
func Scan(pr githubpr.PullRequest, baseRefName string, headSHA string) (*review.Result, string, error) {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		return nil, "", ErrGitleaksNotInstalled
	}

	if baseRefName == "" || headSHA == "" {
		return nil, "", fmt.Errorf(
			"no tengo la rama base o el commit de referencia del pull request para escanear secretos",
		)
	}

	repoDir, err := os.MkdirTemp("", "code-review-gitleaks-*")
	if err != nil {
		return nil, "", fmt.Errorf("no pude crear un directorio temporal: %w", err)
	}
	defer os.RemoveAll(repoDir)

	if err := cloneRepo(pr, repoDir); err != nil {
		return nil, "", err
	}

	if err := fetchPullHead(repoDir, pr.Number); err != nil {
		return nil, "", err
	}

	reportPath := filepath.Join(repoDir, "gitleaks-report.json")

	if err := runGitleaks(repoDir, baseRefName, headSHA, reportPath); err != nil {
		return nil, "", err
	}

	findings, err := readReport(reportPath)
	if err != nil {
		return nil, "", err
	}

	result, rawOutput := buildResult(findings)

	return result, rawOutput, nil
}

// cloneRepo baja el repositorio completo usando `gh`, para reusar la misma
// autenticación que ya usa el resto de la CLI en vez de manejar tokens o
// URLs de git por separado.
func cloneRepo(pr githubpr.PullRequest, destDir string) error {
	cmd := exec.Command("gh", "repo", "clone", pr.Repository(), destDir, "--", "--quiet")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"no pude clonar el repositorio para escanear secretos: %s",
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

// fetchPullHead trae explícitamente el ref del PR. Hace falta cuando el PR
// viene de un fork: el commit de referencia no vive en ninguna rama del
// repositorio base que el clone haya bajado.
func fetchPullHead(repoDir string, number int) error {
	cmd := exec.Command(
		"git", "-C", repoDir, "fetch", "--quiet", "origin",
		fmt.Sprintf("pull/%d/head", number),
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"no pude traer el commit del pull request: %s",
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

// runGitleaks corre gitleaks sobre el historial de commits entre la rama
// base y el commit de referencia del PR. gitleaks devuelve exit code 1
// cuando encuentra secretos: no es un error de ejecución, es la señal
// normal de "hay hallazgos", así que no lo tratamos como fallo.
func runGitleaks(repoDir, baseRefName, headSHA, reportPath string) error {
	cmd := exec.Command(
		"gitleaks",
		"git",
		repoDir,
		"--log-opts=origin/"+baseRefName+".."+headSHA,
		"--report-format=json",
		"--report-path="+reportPath,
		"--redact",
	)

	output, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}

	if err != nil {
		return fmt.Errorf("gitleaks no pudo completar el análisis: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// gitleaksFinding mapea los campos relevantes del reporte JSON de gitleaks.
type gitleaksFinding struct {
	Description string `json:"Description"`
	StartLine   int    `json:"StartLine"`
	File        string `json:"File"`
	Commit      string `json:"Commit"`
	RuleID      string `json:"RuleID"`
}

func readReport(path string) ([]gitleaksFinding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("no pude leer el reporte de gitleaks: %w", err)
	}

	if strings.TrimSpace(string(content)) == "" {
		return nil, nil
	}

	var findings []gitleaksFinding
	if err := json.Unmarshal(content, &findings); err != nil {
		return nil, fmt.Errorf("no pude interpretar el reporte de gitleaks: %w", err)
	}

	return findings, nil
}

func buildResult(findings []gitleaksFinding) (*review.Result, string) {
	reviewFindings := make([]review.Finding, 0, len(findings))

	for _, f := range findings {
		reviewFindings = append(reviewFindings, review.Finding{
			File:    f.File,
			Line:    f.StartLine,
			Title:   fmt.Sprintf("%s (%s)", f.RuleID, shortCommit(f.Commit)),
			Comment: secretComment(f),
		})
	}

	summary := buildSummary(len(reviewFindings))

	return &review.Result{
			Summary:  summary,
			Findings: reviewFindings,
		},
		renderRawSummary(summary, reviewFindings)
}

func secretComment(f gitleaksFinding) string {
	description := strings.TrimSpace(f.Description)
	if description == "" {
		description = "Posible secreto expuesto"
	}

	return fmt.Sprintf("%s en %s:%d (commit %s).", description, f.File, f.StartLine, shortCommit(f.Commit))
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}

	return commit
}

func buildSummary(total int) string {
	if total == 0 {
		return "No encontré secretos expuestos en los commits de este pull request."
	}

	return fmt.Sprintf(
		"Encontré %d posible(s) secreto(s) expuesto(s) en el historial de commits de este pull request.",
		total,
	)
}

func renderRawSummary(summary string, findings []review.Finding) string {
	var out strings.Builder

	out.WriteString("Escaneo de secretos (gitleaks)\n\n")
	out.WriteString(summary)
	out.WriteString("\n")

	if len(findings) == 0 {
		return out.String()
	}

	out.WriteString("\n")

	for _, finding := range findings {
		out.WriteString(fmt.Sprintf("- %s: %s\n", finding.Title, finding.Comment))
	}

	return out.String()
}
