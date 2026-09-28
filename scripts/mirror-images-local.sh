#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Mirror the object store's images to FoxByte's registry, for both architectures.
#
# .github/workflows/mirror-images.yml does this in CI, but it can only ever produce
# amd64: it takes the images out of the published Windows distro image, which is
# built on an amd64 runner, and `docker save`/`load`/`push` keeps one platform.
# Pushed that way, the images run everywhere except Apple Silicon, where the engine
# fails with "exec format error" — which is most of the machines FoxByte is
# developed on.
#
# MinIO's own registry stopped serving anonymous pulls, so the arm64 bytes cannot be
# fetched any more: the only copies are on machines that pulled them while it
# worked. This takes them from there.
#
# Run it inside the engine VM on an arm64 machine whose cache still has them:
#
#   gh auth refresh -h github.com -s write:packages          # once, on the host
#   gh auth token | LIMA_INSTANCE=fox lima bash -c \
#     'cd <repo> && bash scripts/mirror-images-local.sh --amd64-from-registry'
#
# The token is read from stdin so it never appears in a command line.
#
# The tar is the one inside a published release's foxbyte-distro.tar.zst (the
# workflow downloads it; ./usr/local/share/dbengine/images/foxbyte-images.tar). Pass
# --amd64-from-registry instead to reuse the amd64 images the workflow already
# pushed, which is what the plain tags currently hold.
#
# It prints the multi-architecture digests to pin in internal/branch/images.go.
set -euo pipefail

. "$(cd "$(dirname "$0")" && pwd)/lib/brand.sh" 2>/dev/null || true
REGISTRY="${FOX_MIRROR_REGISTRY:-ghcr.io/thefoxbyte}"
IMAGES_GO="$(cd "$(dirname "$0")/.." && pwd)/internal/branch/images.go"

amd64_tar=""
amd64_from_registry=0
while [ $# -gt 0 ]; do
	case "$1" in
	--amd64-tar) amd64_tar="${2:?--amd64-tar needs a path}"; shift 2 ;;
	--amd64-from-registry) amd64_from_registry=1; shift ;;
	-h|--help) sed -n '2,30p' "$0"; exit 0 ;;
	*) echo "unknown argument: $1" >&2; exit 2 ;;
	esac
done
[ -n "$amd64_tar" ] || [ "$amd64_from_registry" = 1 ] || {
	echo "one of --amd64-tar <file> or --amd64-from-registry is required" >&2; exit 2; }

docker() { sudo docker "$@"; }

# The names the engine looks for, read from it rather than repeated here.
go_const() { sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$IMAGES_GO" | head -1; }
minio_tag="$(go_const MinioTag)"; mc_tag="$(go_const MCTag)"
[ -n "$minio_tag" ] && [ -n "$mc_tag" ] || { echo "could not read the image names from $IMAGES_GO" >&2; exit 1; }
echo "destination: $minio_tag"
echo "destination: $mc_tag"

# The token comes in on stdin, or in GH_TOKEN. Prefer stdin: a token on a command
# line is in the shell's history and in every process listing on the machine.
#
#   gh auth token | bash scripts/mirror-images-local.sh --amd64-from-registry
#
# It needs write:packages, which `gh auth refresh -h github.com -s write:packages`
# adds to an existing login.
token="${GH_TOKEN:-}"
if [ -z "$token" ] && [ ! -t 0 ]; then
	token="$(cat)"
fi
token="$(printf '%s' "$token" | tr -d '[:space:]')"
[ -n "$token" ] || {
	echo "a token with write:packages is required, on stdin or in GH_TOKEN:" >&2
	echo "  gh auth refresh -h github.com -s write:packages   # once" >&2
	echo "  gh auth token | bash $0 $*" >&2
	exit 2
}
printf '%s' "$token" | docker login ghcr.io -u "${GH_USER:-x}" --password-stdin >/dev/null || {
	echo "that token was refused by ghcr.io — does it have write:packages?" >&2; exit 1; }
unset token

# The arm64 copies: whatever is in this machine's cache for each image, whichever
# registry it came from. Found by repository suffix, as the workflow does.
find_local() { docker images --format '{{.ID}} {{.Repository}}' | awk -v n="$1" '$2 ~ "/" n "$" {print $1; exit}'; }
arch_of() { docker image inspect "$1" --format '{{.Architecture}}'; }

push_arch() { # <image id or ref> <destination tag> <arch> -> pushes <tag>-<arch>
	local src="$1" dest="$2" want="$3" got
	got="$(arch_of "$src")"
	[ "$got" = "$want" ] || { echo "expected $want, but $src is $got" >&2; return 1; }
	docker tag "$src" "${dest}-${want}"
	docker push -q "${dest}-${want}" >&2
	echo "${dest}-${want}"
}

echo "=== arm64, from this machine's cache ==="
for pair in "minio:$minio_tag" "mc:$mc_tag"; do
	name="${pair%%:*}"; dest="${pair#*:}"
	id="$(find_local "$name")"
	[ -n "$id" ] || { echo "no local copy of an image whose repository ends in /$name" >&2; exit 1; }
	push_arch "$id" "$dest" arm64 >/dev/null
	echo "  pushed ${dest}-arm64"
done

echo "=== amd64 ==="
if [ -n "$amd64_tar" ]; then
	docker load -i "$amd64_tar" >/dev/null
	for pair in "minio:$minio_tag" "mc:$mc_tag"; do
		name="${pair%%:*}"; dest="${pair#*:}"
		# The freshly loaded amd64 copy, not the arm64 one already here.
		id=""
		for candidate in $(docker images --format '{{.ID}} {{.Repository}}' | awk -v n="$name" '$2 ~ "/" n "$" {print $1}'); do
			[ "$(arch_of "$candidate")" = amd64 ] && { id="$candidate"; break; }
		done
		[ -n "$id" ] || { echo "the tar holds no amd64 copy of /$name" >&2; exit 1; }
		push_arch "$id" "$dest" amd64 >/dev/null
		echo "  pushed ${dest}-amd64"
	done
else
	# What the workflow already pushed under the plain tag is the amd64 build.
	for dest in "$minio_tag" "$mc_tag"; do
		docker buildx imagetools create -t "${dest}-amd64" "$dest" >/dev/null
		echo "  copied ${dest}-amd64 from what is already published"
	done
fi

echo "=== combine into one multi-architecture tag ==="
for dest in "$minio_tag" "$mc_tag"; do
	docker buildx imagetools create -t "$dest" "${dest}-amd64" "${dest}-arm64" >/dev/null
	digest="$(docker buildx imagetools inspect "$dest" --format '{{.Manifest.Digest}}' | tail -n1 | tr -d '[:space:]')"
	platforms="$(docker buildx imagetools inspect "$dest" --raw | python3 -c 'import sys,json;d=json.load(sys.stdin);print(", ".join(m["platform"]["os"]+"/"+m["platform"]["architecture"] for m in d.get("manifests",[])) or "single platform")')"
	printf '%s\n  %s\n  platforms: %s\n' "$dest" "$digest" "$platforms"
done

echo
echo "Put those digests in internal/branch/images.go (MinioDigest, MCDigest)."
