#!/usr/bin/env bash
set -euo pipefail

# Mirrors the public release onto the same server that serves api.ciyuanshen.top.
# Files are published before update.json, so clients never see a missing package.
readonly repository='China-520-1314/ciyuanshen-config-assistant'
readonly release_api="https://api.github.com/repos/${repository}/releases/latest"
readonly destination_root='/var/lib/ciyuanshen-config-assistant-downloads'
readonly public_base='https://api.ciyuanshen.top/downloads/ciyuanshen-config-assistant'
readonly installer_name='ciyuanshen-config-assistant-amd64-installer.exe'
readonly mac_dmg_name='ciyuanshen-config-assistant-macos-universal.dmg'
readonly mac_zip_name='ciyuanshen-config-assistant-macos-universal.zip'

log() {
  printf '[release-sync] %s\n' "$*" >&2
}

fail() {
  log "error: $*"
  exit 1
}

for command in curl flock jq sha256sum; do
  command -v "$command" >/dev/null 2>&1 || fail "missing required command: $command"
done

install -d -m 0755 "$destination_root/releases"
exec 9>"/run/lock/ciyuanshen-config-assistant-release-sync.lock"
flock -n 9 || exit 0

work_dir=$(mktemp -d "$destination_root/.sync.XXXXXX")
cleanup() {
  rm -rf -- "$work_dir"
}
trap cleanup EXIT

release_json="$work_dir/release.json"
curl --fail --location --silent --show-error --retry 3 --retry-delay 3 --connect-timeout 20 --max-time 120 \
  --header 'Accept: application/vnd.github+json' \
  --header 'User-Agent: CiyuanShen-Config-Assistant-Release-Sync' \
  --output "$release_json" "$release_api"

tag=$(jq -er '.tag_name' "$release_json")
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "latest release tag is not a stable semantic version: $tag"
version=${tag#v}
published_at=$(jq -er '.published_at' "$release_json")
published_date=${published_at%%T*}
release_directory="$destination_root/releases/$tag"

if [[ -f "$destination_root/update.json" ]]; then
  current_version=$(jq -r '.version // empty' "$destination_root/update.json" 2>/dev/null || true)
  if [[ "$current_version" == "$version" ]]; then
    complete_release=1
    for asset in "$installer_name" "$mac_dmg_name" "$mac_zip_name"; do
      [[ -f "$release_directory/$asset" ]] || complete_release=0
    done
    if [[ "$complete_release" == 1 ]]; then
      log "latest release $tag is already mirrored"
      exit 0
    fi
  fi
fi

asset_url() {
  jq -er --arg name "$1" '[.assets[] | select(.name == $name) | .browser_download_url][0] // empty' "$release_json"
}

download_asset() {
  local url=$1
  local output=$2
  curl --fail --location --silent --show-error --retry 3 --retry-delay 3 --connect-timeout 20 --max-time 1200 --output "$output" "$url"
}

installer_url=$(asset_url "$installer_name")
mac_dmg_url=$(asset_url "$mac_dmg_name")
mac_zip_url=$(asset_url "$mac_zip_name")
source_manifest_url=$(asset_url 'update.json')

download_asset "$installer_url" "$work_dir/$installer_name"
download_asset "$mac_dmg_url" "$work_dir/$mac_dmg_name"
download_asset "$mac_zip_url" "$work_dir/$mac_zip_name"
download_asset "$source_manifest_url" "$work_dir/source-update.json"

manifest_version=$(jq -er '.version' "$work_dir/source-update.json")
[[ "$manifest_version" == "$version" ]] || fail "GitHub manifest version $manifest_version does not match release $version"
installer_sha=$(jq -er '.sha256' "$work_dir/source-update.json" | tr '[:upper:]' '[:lower:]')
[[ "$installer_sha" =~ ^[0-9a-f]{64}$ ]] || fail 'GitHub manifest does not contain a valid installer SHA256'
actual_sha=$(sha256sum "$work_dir/$installer_name" | awk '{print $1}')
[[ "$actual_sha" == "$installer_sha" ]] || fail 'installer SHA256 does not match the GitHub manifest'

if [[ ! -d "$release_directory" ]]; then
  stage_directory=$(mktemp -d "$destination_root/releases/.${tag}.XXXXXX")
  chmod 0755 "$stage_directory"
  install -m 0644 "$work_dir/$installer_name" "$stage_directory/$installer_name"
  install -m 0644 "$work_dir/$mac_dmg_name" "$stage_directory/$mac_dmg_name"
  install -m 0644 "$work_dir/$mac_zip_name" "$stage_directory/$mac_zip_name"
  mv "$stage_directory" "$release_directory"
fi

for asset in "$installer_name" "$mac_dmg_name" "$mac_zip_name"; do
  [[ -f "$release_directory/$asset" ]] || fail "published release is missing $asset"
done

manifest_temp="$destination_root/.update.json.$$"
jq -n \
  --arg version "$version" \
  --arg download_url "$public_base/releases/$tag/$installer_name" \
  --arg mac_download_url "$public_base/releases/$tag/$mac_dmg_name" \
  --arg published_at "$published_date" \
  --arg sha256 "$installer_sha" \
  '{version: $version, downloadUrl: $download_url, macDownloadUrl: $mac_download_url, releaseNotes: "请查看本次版本更新说明", publishedAt: $published_at, sha256: $sha256}' > "$manifest_temp"
chmod 0644 "$manifest_temp"
mv -f "$manifest_temp" "$destination_root/update.json"

log "published $tag to $public_base"
