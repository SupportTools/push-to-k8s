#!/bin/sh
# apk add through the org fetch-cache, with a logged upstream fallback.
# Used by both alpine stages of the Dockerfile through a RUN bind mount, so it
# never lands in an image layer. cluster-services-972.
#
#   APK_MIRROR    base URL that serves dl-cdn.alpinelinux.org's /alpine/ tree,
#                 e.g. http://fetch-cache.fetch-cache.svc.a1-ops-prd.local/alpine
#                 (fetch tier: a cache we run, cluster-services-854).
#                 Empty = upstream dl-cdn.alpinelinux.org, no mirror attempt.
#   APK_FALLBACK  "upstream" (default): if the mirror attempt fails, restore the
#                 base image's upstream repositories and retry once, loudly.
#                 "none": fail instead. CI uses this so a build that silently
#                 fell back cannot pass as proof that the mirror works.
#
# Plain HTTP to the cache loses no integrity: apk verifies every index and
# package signature against the keys baked into the base image.
#
# The rewrite only swaps the host prefix, so the Alpine release (v3.20 today)
# comes from the base image itself and an ALPINE_IMAGE bump needs no edit here.
# /etc/apk/repositories is restored before exit, so the shipped image names no
# in-cluster host.
set -eu

REPOS=/etc/apk/repositories
UPSTREAM_SAVE=/tmp/apk-repositories.upstream
UPSTREAM_PREFIX='https://dl-cdn.alpinelinux.org/alpine/'
fallback="${APK_FALLBACK:-upstream}"

case "$fallback" in
  upstream|none) ;;
  *) echo "apk-add: APK_FALLBACK must be 'upstream' or 'none', got '$fallback'" >&2; exit 2 ;;
esac

cp "$REPOS" "$UPSTREAM_SAVE"
restore() { cp "$UPSTREAM_SAVE" "$REPOS"; rm -f "$UPSTREAM_SAVE"; }
trap restore EXIT

if [ -z "${APK_MIRROR:-}" ]; then
  echo "apk source: upstream (APK_MIRROR empty)"
  apk add --no-cache "$@"
  exit 0
fi

mirror="${APK_MIRROR%/}/"
sed -i "s#^${UPSTREAM_PREFIX}#${mirror}#" "$REPOS"
# A rewrite that matched nothing would quietly fetch upstream, which is exactly
# what APK_MIRROR exists to stop. Treat it as a build bug, not a fallback case.
if grep -q 'dl-cdn.alpinelinux.org' "$REPOS" || ! grep -q "^${mirror}" "$REPOS"; then
  echo "apk-add: could not point $REPOS at $mirror; it reads:" >&2
  cat "$REPOS" >&2
  exit 1
fi

echo "apk source: mirror $mirror"
# --timeout bounds a mirror that accepts connections but never answers, so the
# fallback is reached in seconds rather than at the job timeout.
if apk --timeout 30 add --no-cache "$@"; then
  exit 0
fi

if [ "$fallback" = none ]; then
  echo "apk-add: mirror $mirror failed and APK_FALLBACK=none" >&2
  exit 1
fi
echo "apk source: MIRROR FAILED ($mirror), falling back to upstream dl-cdn.alpinelinux.org" >&2
cp "$UPSTREAM_SAVE" "$REPOS"
apk add --no-cache "$@"
