package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallMethod describe CÓMO se instaló el binario en disco. Es la
// base para que uninstall sepa si puede borrar a ciegas o tiene que
// pedirle al package manager que lo haga.
type InstallMethod string

const (
	InstallMethodLocal    InstallMethod = "local"   // ~/.local/bin (install.sh)
	InstallMethodGoBin    InstallMethod = "gobin"   // $GOBIN o $GOPATH/bin (go install)
	InstallMethodHomebrew InstallMethod = "brew"    // /opt/homebrew/bin o /usr/local/bin vía brew
	InstallMethodUnknown  InstallMethod = "unknown" // no pudimos determinarlo
)

// UninstallResult lleva la metadata que los comandos necesitan para
// mostrar un mensaje útil al usuario después del borrado.
type UninstallResult struct {
	Method InstallMethod
	Path   string
}

// ErrCannotUninstall se devuelve cuando el binario vive en un lugar
// que la CLI no debe tocar (Homebrew, /usr/bin, etc.). El caller lo
// traduce a un mensaje instructivo.
var ErrCannotUninstall = errors.New("cannot uninstall from this location")

// DetectInstallMethod inspecciona el path del binario actual (resuelto
// con os.Executable) y decide bajo qué InstallMethod cae.
//
// Reglas:
//   - path bajo $HOME/.local/bin                  -> local
//   - path bajo $GOBIN o $GOPATH/bin              -> gobin
//   - path bajo una cellar de Homebrew            -> brew
//   - cualquier otra cosa                         -> unknown
//
// Unknown NO es un error: es señal de que uninstall no debe borrar a
// ciegas y debe pedirle al usuario que use el package manager
// correspondiente.
func DetectInstallMethod() (InstallMethod, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return InstallMethodUnknown, "", fmt.Errorf("could not resolve current binary path: %w", err)
	}

	// os.Executable puede devolver un path con symlinks resueltos o no,
	// según el SO. Resolvemos para tener una comparación estable.
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = resolved
	}

	switch {
	case isUnderHomebrew(exe):
		return InstallMethodHomebrew, exe, nil

	case isUnderLocalBin(exe):
		return InstallMethodLocal, exe, nil

	case isUnderGoBin(exe):
		return InstallMethodGoBin, exe, nil

	default:
		return InstallMethodUnknown, exe, nil
	}
}

// Uninstall borra el binario si el método de instalación lo permite.
// Para Homebrew devuelve ErrCannotUninstall con la instrucción de
// `brew uninstall`. Para Unknown hace lo mismo y le pide al usuario
// que borre manualmente.
func Uninstall() (UninstallResult, error) {
	method, path, err := DetectInstallMethod()
	if err != nil {
		return UninstallResult{}, err
	}

	switch method {
	case InstallMethodLocal, InstallMethodGoBin:
		if err := os.Remove(path); err != nil {
			return UninstallResult{}, fmt.Errorf("could not remove %s: %w", path, err)
		}
		return UninstallResult{Method: method, Path: path}, nil

	case InstallMethodHomebrew:
		return UninstallResult{Method: method, Path: path},
			fmt.Errorf("%w: installed via Homebrew. Run `brew uninstall %s`",
				ErrCannotUninstall, brewFormulaName())

	default:
		return UninstallResult{Method: method, Path: path},
			fmt.Errorf("%w: cannot determine how %s was installed; remove it manually",
				ErrCannotUninstall, path)
	}
}

// isUnderHomebrew considera como Homebrew tanto Apple Silicon
// (/opt/homebrew) como Intel (/usr/local).
func isUnderHomebrew(path string) bool {
	prefixes := []string{
		"/opt/homebrew/bin/",
		"/opt/homebrew/sbin/",
		"/usr/local/Cellar/",
		"/usr/local/bin/",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func isUnderLocalBin(path string) bool {
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".local", "bin")
		if strings.HasPrefix(path, dir+string(filepath.Separator)) || path == dir {
			return true
		}
	}
	return false
}

func isUnderGoBin(path string) bool {
	candidates := []string{}
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		candidates = append(candidates, gobin)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "bin"))
	} else {
		// Default: ~/go/bin
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "go", "bin"))
		}
	}

	cwd, _ := os.Getwd()
	_ = cwd // (silence unused warning en plataformas que no la usan)

	for _, c := range candidates {
		if c == "" {
			continue
		}
		c = filepath.Clean(c)
		if strings.HasPrefix(path, c+string(filepath.Separator)) || path == c {
			return true
		}
	}
	return false
}

// brewFormulaName devuelve el nombre de la fórmula en Homebrew. Como
// todavía no tenemos un tap oficial, devuelve el nombre del binario
// como mejor aproximación; si el usuario tiene un nombre distinto
// puede sobrescribirlo con HOMEBREW_FORMULA_NAME.
func brewFormulaName() string {
	if v := os.Getenv("HOMEBREW_FORMULA_NAME"); v != "" {
		return v
	}
	return "code-review"
}

// CanSelfUpgrade es un helper expuesto para los comandos: dice si el
// método de instalación es uno donde upgrade/downgrade (que hacen
// replace del binario actual) son seguros de hacer.
//
// Hoy: local y gobin sí. brew y unknown no.
func CanSelfUpgrade() bool {
	method, _, err := DetectInstallMethod()
	if err != nil {
		return false
	}
	switch method {
	case InstallMethodLocal, InstallMethodGoBin:
		return true
	default:
		return false
	}
}

// SelfUpgradeDisabledReason devuelve el motivo por el cual no se
// puede hacer self-upgrade en este entorno. Es "" si sí se puede.
func SelfUpgradeDisabledReason() string {
	method, path, err := DetectInstallMethod()
	if err != nil {
		return err.Error()
	}
	switch method {
	case InstallMethodLocal, InstallMethodGoBin:
		return ""
	case InstallMethodHomebrew:
		return "code-review was installed via Homebrew; run `brew upgrade " + brewFormulaName() + "` instead"
	default:
		return "code-review is installed at " + path +
			" which is not a self-updatable location; use the install script at " +
			HostOf() + "/blob/master/scripts/install.sh"
	}
}

// Sanity-check runtime: si por alguna razón estamos en js/wasm o algo
// raro, runtime.GOOS da "" y todo se rompe. Mejor cortar antes.
func init() {
	if runtime.GOOS == "" {
		panic("updater: runtime.GOOS is empty")
	}
	_ = exec.Command // import keep
}
