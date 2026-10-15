#!/bin/sh
# Installs the rierek/lif project's lif and lif-serve
# binaries from a GitHub release.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rierek/lif/main/install.sh | sh
#
# Configuration (environment variables):
#   LIF_VERSION  Release tag to install, e.g. "v0.7.0". Default: latest release.
#   LIF_BIN_DIR  Install directory. Default: "$HOME/.local/bin".
#   LIF_REPO     GitHub "owner/repo" to install from. Default: "rierek/lif".
set -eu

LIF_REPO="${LIF_REPO:-rierek/lif}"
LIF_BIN_DIR="${LIF_BIN_DIR:-$HOME/.local/bin}"

err() {
	echo "install.sh: $*" >&2
	exit 1
}

info() {
	echo "install.sh: $*" >&2
}

resolve_version() {
	if [ -n "${LIF_VERSION:-}" ]; then
		echo "$LIF_VERSION"
		return
	fi
	api_url="https://api.github.com/repos/${LIF_REPO}/releases/latest"
	tag=$(curl -fsSL "$api_url" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
	if [ -z "$tag" ]; then
		err "could not resolve the latest release tag from $api_url. Set LIF_VERSION explicitly (e.g. LIF_VERSION=v0.7.0)."
	fi
	echo "$tag"
}

detect_os() {
	uname_s=$(uname -s)
	case "$uname_s" in
	Darwin) echo "Darwin" ;;
	Linux) echo "Linux" ;;
	*)
		err "unsupported OS '$uname_s'. Windows users: download the .zip archive from https://github.com/${LIF_REPO}/releases instead."
		;;
	esac
}

detect_arch() {
	os="$1"
	uname_m=$(uname -m)
	case "$uname_m" in
	x86_64 | amd64) echo "x86_64" ;;
	arm64 | aarch64) echo "arm64" ;;
	i386 | i686)
		if [ "$os" = "Linux" ]; then
			echo "i386"
		else
			err "unsupported architecture 'i386' for OS '$os'."
		fi
		;;
	*)
		err "unsupported architecture '$uname_m'. Supported: x86_64, arm64 (and i386 on Linux)."
		;;
	esac
}

sha256_check() {
	file="$1"
	checksums="$2"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$(dirname "$file")" && grep " $(basename "$file")\$" "$checksums" | sha256sum -c -)
	elif command -v shasum >/dev/null 2>&1; then
		(cd "$(dirname "$file")" && grep " $(basename "$file")\$" "$checksums" | shasum -a 256 -c -)
	else
		info "warning: neither sha256sum nor shasum found, skipping checksum verification."
		return 0
	fi
}

main() {
	version=$(resolve_version)
	os=$(detect_os)
	arch=$(detect_arch "$os")
	archive="lif_${os}_${arch}.tar.gz"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	base_url="https://github.com/${LIF_REPO}/releases/download/${version}"
	info "downloading ${archive} (${version})..."
	curl -fsSL -o "$tmp/$archive" "${base_url}/${archive}" ||
		err "failed to download ${base_url}/${archive}. Check that ${version} exists at https://github.com/${LIF_REPO}/releases."

	if curl -fsSL -o "$tmp/checksums.txt" "${base_url}/checksums.txt"; then
		sha256_check "$tmp/$archive" "$tmp/checksums.txt" || err "checksum verification failed for ${archive}."
	else
		info "warning: could not download checksums.txt, skipping checksum verification."
	fi

	tar -xzf "$tmp/$archive" -C "$tmp"

	mkdir -p "$LIF_BIN_DIR"
	install -m 755 "$tmp/lif" "$tmp/lif-serve" "$LIF_BIN_DIR/" ||
		err "failed to install binaries into ${LIF_BIN_DIR}. Set LIF_BIN_DIR to a writable directory."

	"$LIF_BIN_DIR/lif" version

	case ":$PATH:" in
	*":$LIF_BIN_DIR:"*) ;;
	*)
		info "note: ${LIF_BIN_DIR} is not on your PATH. Add this to your shell configuration:"
		info "  export PATH=\"${LIF_BIN_DIR}:\$PATH\""
		;;
	esac
}

main "$@"
