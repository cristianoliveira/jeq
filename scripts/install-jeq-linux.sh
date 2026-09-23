#!/usr/bin/env bash
set -euo pipefail

version="${1:-${JEQ_VERSION:-v0.1.0-rc.2}}"
install_dir="${2:-${INSTALL_DIR:-$HOME/.local/bin}}"

case "$(uname -s)" in
  Linux) ;;
  *)
    printf 'error: this installer supports Linux only\n' >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *)
    printf 'error: unsupported Linux architecture: %s\n' "$(uname -m)" >&2
    exit 1
    ;;
esac

version_number="${version#v}"
archive="jeq_${version_number}_linux_${arch}.tar.gz"
base_url="https://github.com/cristianoliveira/jeq/releases/download/${version}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

curl --fail --silent --show-error --location \
  --output "$tmp_dir/$archive" "$base_url/$archive"
curl --fail --silent --show-error --location \
  --output "$tmp_dir/checksums.txt" "$base_url/checksums.txt"

(
  cd "$tmp_dir"
  grep "  $archive$" checksums.txt | sha256sum --check --status
)

mkdir -p "$install_dir"
tar --extract --gzip --file "$tmp_dir/$archive" --directory "$tmp_dir"
install -m 0755 "$tmp_dir/jeq" "$install_dir/jeq"
printf 'Installed jeq %s to %s/jeq\n' "$version" "$install_dir"
