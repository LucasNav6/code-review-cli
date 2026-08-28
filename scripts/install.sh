#!/usr/bin/env sh
# Instala el último binario de code-review publicado en GitHub Releases,
# sin necesidad de tener Go instalado. Pensado para usarse como:
#
#   curl -fsSL https://raw.githubusercontent.com/LucasNav6/code-review-cli/master/scripts/install.sh | sh
#
set -eu

# Se puede apuntar a un fork propio con CODE_REVIEW_REPO=owner/repo.
REPO="${CODE_REVIEW_REPO:-LucasNav6/code-review-cli}"
INSTALL_DIR="${CODE_REVIEW_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${CODE_REVIEW_VERSION:-latest}"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
    Linux) goos="linux" ;;
    Darwin) goos="darwin" ;;
    *)
        echo "Sistema operativo no soportado: $os" >&2
        exit 1
        ;;
esac

case "$arch" in
    x86_64 | amd64) goarch="amd64" ;;
    arm64 | aarch64) goarch="arm64" ;;
    *)
        echo "Arquitectura no soportada: $arch" >&2
        exit 1
        ;;
esac

asset="code-review_${goos}_${goarch}.tar.gz"

if [ "$VERSION" = "latest" ]; then
    asset_url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
    asset_url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Descargando code-review ($goos/$goarch) desde $REPO..."
curl -fsSL "$asset_url" -o "$tmp_dir/code-review.tar.gz"

tar -xzf "$tmp_dir/code-review.tar.gz" -C "$tmp_dir" code-review

mkdir -p "$INSTALL_DIR"
mv "$tmp_dir/code-review" "$INSTALL_DIR/code-review"
chmod +x "$INSTALL_DIR/code-review"

echo "code-review instalado en $INSTALL_DIR/code-review"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "Agregá $INSTALL_DIR a tu PATH para poder usar \"code-review\" directamente, por ejemplo:"
        echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc"
        ;;
esac
