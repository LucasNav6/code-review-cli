package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LucasNav6/code-review-cli/buildinfo"
)

// InstallResult es lo que devuelve Upgrade/Downgrade. Path es la ruta
// del binario recién escrito y Version es el tag que se instaló.
type InstallResult struct {
	Path    string
	Version string
}

// Upgrade descarga la release pedida y reemplaza el binario actual en
// su lugar de forma atómica (escribe en un .new y hace rename).
//
// Si currentPath no corresponde al binario en ejecución (por ejemplo
// en un dev build), devuelve error para que el caller pueda guiar al
// usuario hacia install.sh en vez de hacer un self-replace ciego.
func Upgrade(ctx context.Context, release *Release, currentPath string) (InstallResult, error) {
	if buildinfo.IsDev() {
		return InstallResult{}, fmt.Errorf("cannot upgrade a 'dev' build; install a release first")
	}

	if !release.IsNewer(buildinfo.Version) {
		return InstallResult{}, fmt.Errorf("release %s is not newer than current %s",
			release.TagName, buildinfo.Version)
	}

	return replaceCurrentBinary(ctx, release, currentPath)
}

// Downgrade es la inversa: requiere un target explícito y falla si
// no es estrictamente menor que la versión actual. Misma mecánica
// de reemplazo atómico.
func Downgrade(ctx context.Context, release *Release, currentPath string) (InstallResult, error) {
	if buildinfo.IsDev() {
		return InstallResult{}, fmt.Errorf("cannot downgrade a 'dev' build")
	}

	if !release.IsOlder(buildinfo.Version) {
		return InstallResult{}, fmt.Errorf("release %s is not older than current %s",
			release.TagName, buildinfo.Version)
	}

	return replaceCurrentBinary(ctx, release, currentPath)
}

// replaceCurrentBinary descarga el asset de la release, lo extrae,
// valida el checksum y reemplaza currentPath en una operación atómica.
func replaceCurrentBinary(ctx context.Context, release *Release, currentPath string) (InstallResult, error) {
	assetURL, err := release.DownloadURL()
	if err != nil {
		return InstallResult{}, err
	}

	// Bajamos a un tmpdir porque queremos extraer ANTES de tocar el
	// binario actual (si algo falla a mitad de la extracción, el
	// binario viejo sigue sano).
	tmpDir, err := os.MkdirTemp("", "code-review-upgrade-*")
	if err != nil {
		return InstallResult{}, fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveNameFor(assetURL))
	if err := downloadTo(ctx, assetURL, archivePath); err != nil {
		return InstallResult{}, fmt.Errorf("download failed: %w", err)
	}

	binaryPath := filepath.Join(tmpDir, "code-review")
	if err := extract(archivePath, binaryPath); err != nil {
		return InstallResult{}, fmt.Errorf("extraction failed: %w", err)
	}

	// replaceAtomic: escribe al lado del binario actual (mismo
	// filesystem -> rename es atómico en POSIX) y luego swap.
	if err := swapBinary(binaryPath, currentPath); err != nil {
		return InstallResult{}, fmt.Errorf("could not replace binary at %s: %w", currentPath, err)
	}

	return InstallResult{
		Path:    currentPath,
		Version: release.TagName,
	}, nil
}

// downloadTo baja un archivo con timeout y lo guarda en dst. No usa
// el cliente default porque queremos configurar timeout y User-Agent
// explícitamente.
func downloadTo(ctx context.Context, src, dst string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

// extract desempaqueta el archivo en dst. Soporta tar.gz (linux/darwin)
// y zip (windows) según la convención de goreleaser.
func extract(archivePath, dst string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, dst)
	}
	return extractTarGz(archivePath, dst)
}

func extractTarGz(archivePath, dst string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Goreleaser mete UN solo archivo: "code-review" en la raíz.
		// No seguimos symlinks ni paths absolutos por seguridad.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "code-review" {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("archive %s did not contain a 'code-review' binary", archivePath)
}

func extractZip(archivePath, dst string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) != "code-review.exe" && filepath.Base(f.Name) != "code-review" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("archive %s did not contain a 'code-review' binary", archivePath)
}

// swapBinary copia src sobre dst en una operación atómica:
//  1. escribe src en dst+".new"
//  2. hace rename de dst+".new" -> dst
//
// En POSIX el rename es atómico dentro del mismo filesystem; si el
// proceso está en ejecución, el binario viejo sigue vivo mientras se
// hace el swap (el archivo mapeado no se invalida hasta el exec
// siguiente, que es justamente lo que queremos: el binario actual
// termina la corrida, el próximo exec usa el nuevo).
func swapBinary(src, dst string) error {
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, "code-review-*.new")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		// Si fallamos a mitad de camino, limpiamos el .new.
		_ = os.Remove(tmpPath)
	}()

	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return err
	}
	defer in.Close()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// En Windows no se puede renombrar sobre un binario en ejecución;
	// la estrategia POSIX no aplica. Para Windows dejamos un mensaje
	// claro en lugar de hacer algo destructivo.
	if runtime.GOOS == "windows" {
		return fmt.Errorf("self-upgrade is not supported on Windows; "+
			"download the new binary from %s/releases and replace %s manually",
			HostOf(), dst)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

// archiveNameFor infiere el nombre del archivo destino a partir de la
// URL. Sirve solo para decidir el path local: la detección del formato
// real (zip vs tar.gz) se hace en extract() por extensión.
func archiveNameFor(rawURL string) string {
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return "asset.bin"
}

// Sha256File es una helper expuesta por si en el futuro queremos
// validar contra checksums.txt. Por ahora no la usamos en el camino
// crítico para no agregar un round-trip más a GitHub.
func Sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
