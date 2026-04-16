#!/bin/sh
# pad installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/prefapp/pad/main/scripts/install.sh | sh

set -e

REPO="prefapp/pad"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS and architecture
OS=$(uname -s)
ARCH=$(uname -m)

case "$OS" in
    Linux)
        GOOS="Linux"
        ;;
    Darwin)
        GOOS="Darwin"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        GOOS="Windows"
        ;;
    *)
        echo "Error: Unsupported operating system: $OS" >&2
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        GOARCH="x86_64"
        ;;
    arm64|aarch64)
        GOARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# Get latest release version
echo "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
    echo "Error: Could not determine latest release" >&2
    exit 1
fi

echo "Latest version: $LATEST"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download release
echo "Downloading pad for $GOOS/$GOARCH..."
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST/pad_${GOOS}_${GOARCH}.tar.gz"

if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/pad.tar.gz"; then
    echo "Error: Failed to download $DOWNLOAD_URL" >&2
    exit 1
fi

# Extract
echo "Extracting..."
tar -xzf "$TMP_DIR/pad.tar.gz" -C "$TMP_DIR"

# Create install directory if needed
if [ ! -d "$INSTALL_DIR" ]; then
    echo "Creating $INSTALL_DIR..."
    mkdir -p "$INSTALL_DIR"
fi

# Check if directory is in PATH
case ":$PATH:" in
    *":$INSTALL_DIR:"*|*":${INSTALL_DIR}/:"*)
        # Directory is in PATH
        ;;
    *)
        echo ""
        echo "Warning: $INSTALL_DIR is not in your PATH."
        echo "Add this to your shell profile:"
        echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        echo ""
        ;;
esac

# Install binary
echo "Installing to $INSTALL_DIR/pad..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/pad" "$INSTALL_DIR/pad"
    chmod +x "$INSTALL_DIR/pad"
else
    echo "Need sudo access to install to $INSTALL_DIR"
    sudo mv "$TMP_DIR/pad" "$INSTALL_DIR/pad"
    sudo chmod +x "$INSTALL_DIR/pad"
fi

# Verify installation
if command -v pad >/dev/null 2>&1; then
    INSTALLED_VERSION=$(pad --version)
    echo ""
    echo "✓ pad $INSTALLED_VERSION installed successfully!"
    echo ""
    echo "Get started with:"
    echo "  pad init"
else
    echo ""
    echo "✓ pad installed to $INSTALL_DIR/pad"
    echo ""
    echo "Make sure $INSTALL_DIR is in your PATH, then run:"
    echo "  pad init"
fi
