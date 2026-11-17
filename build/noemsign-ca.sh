#!/bin/sh
# noemsign-ca.sh: strip every eMudhra/emSign root from a container root filesystem, and guard that none
# is left (cluster-services-1392). Fleet CA standard, operator decision 2026-11-13 (cluster-services-1375):
# emSign Root CA - G1, emSign ECC Root CA - G3, emSign Root CA - C1 and emSign ECC Root CA - C3 must all
# be absent from every trust store an image ships.
#
# Canonical copy: SupportTools/cluster-services noemsign/noemsign-ca.sh. Repos that build images vendor an
# identical copy (same sha256) next to their Dockerfile; update the canonical one first, then the copies.
#
# Usage (POSIX sh; needs find, grep, awk, openssl):
#   noemsign-ca.sh strip ROOT   remove/filter emSign certificates in ROOT (in place)
#   noemsign-ca.sh guard ROOT   exit 1 if any emSign certificate is left in ROOT
#
# strip:
#   - Debian/Ubuntu (ROOT=/ with /etc/ca-certificates.conf and update-ca-certificates): deselects every
#     emSign entry ("!mozilla/emSign_...") and runs update-ca-certificates --fresh, so a later
#     ca-certificates upgrade inside the image keeps them out.
#   - Every regular file under ROOT that holds PEM certificates: a single emSign certificate is deleted, a
#     bundle is rewritten without its emSign certificates (system bundle, cert.pem, *.apk-new leftovers,
#     certifi and other language bundles alike). Matching is on the x509-decoded subject, never the name.
#   - Symlinks left dangling under the usual trust directories (hash links, ca-cert-*.pem) are removed.
# guard: decodes the subject of every PEM certificate in every regular file under ROOT, and every
#   DER certificate file in the trust directories, and fails on any emSign/eMudhra subject. It also fails
#   if ROOT/etc/ssl/certs/ca-certificates.crt exists with fewer than 100 certificates (a truncated bundle
#   must not ship). Non-certificate files that merely contain the string "emSign" (a compiled-in root in a
#   binary, a changelog) are listed as WARN lines, not failures.
#
# /proc, /sys and /dev under ROOT are skipped; with ROOT=/ the scan stays on the root filesystem (-xdev).
set -eu

mode=${1:?usage: noemsign-ca.sh strip|guard ROOT}
root=${2:?usage: noemsign-ca.sh strip|guard ROOT}
root=${root%/}
[ -n "$root" ] || root=/
pat='emsign|emudhra'

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

# Regular files under ROOT that contain a PEM certificate.
pem_files() {
  find "$root" -xdev \( -path "${root%/}/proc" -o -path "${root%/}/sys" -o -path "${root%/}/dev" \) -prune \
    -o -path "$work" -prune -o -type f -size -20000k -exec grep -l -e '-----BEGIN CERTIFICATE-----' {} + 2>/dev/null || true
}

# split FILE DIR: one PEM per certificate, DIR/c0001.pem ...
split() {
  rm -rf "$2"; mkdir -p "$2"
  awk -v d="$2" '/-----BEGIN CERTIFICATE-----/{n++; f=sprintf("%s/c%04d.pem", d, n)} f{print > f} /-----END CERTIFICATE-----/{close(f); f=""}' "$1"
}

subject() { openssl x509 -in "$1" -noout -subject -nameopt RFC2253 2>/dev/null || echo "subject=UNPARSEABLE"; }

trust_dirs="etc/ssl etc/pki usr/share/ca-certificates usr/local/share/ca-certificates etc/ca-certificates usr/lib/ssl"

strip() {
  if [ "$root" = / ] && [ -f /etc/ca-certificates.conf ] && command -v update-ca-certificates >/dev/null 2>&1; then
    sed -i -E 's|^(mozilla/emSign[^[:space:]]*)$|!\1|' /etc/ca-certificates.conf
    if grep -Eiq '^mozilla/.*emsign' /etc/ca-certificates.conf; then echo "noemsign-ca: conf still selects emSign" >&2; exit 1; fi
    update-ca-certificates --fresh >/dev/null
    echo "noemsign-ca: deselected $(grep -Eic '^!mozilla/.*emsign' /etc/ca-certificates.conf || true) emSign entries in /etc/ca-certificates.conf"
  fi
  pem_files > "$work/files"
  while IFS= read -r f; do
    split "$f" "$work/s"
    total=0; bad=0
    for c in "$work"/s/c*.pem; do
      [ -e "$c" ] || continue
      total=$((total + 1))
      if subject "$c" | grep -Eiq "$pat"; then bad=$((bad + 1)); echo "noemsign-ca: $f: drop $(subject "$c")"; fi
    done
    [ "$bad" -gt 0 ] || continue
    if [ "$bad" -eq "$total" ]; then
      rm -f "$f"; echo "noemsign-ca: removed $f"
    else
      # Rewrite the bundle in place, keeping only the non-emSign certificates (comments are dropped).
      : > "$work/out"
      for c in "$work"/s/c*.pem; do subject "$c" | grep -Eiq "$pat" || cat "$c" >> "$work/out"; done
      cat "$work/out" > "$f"
      echo "noemsign-ca: $f: $total -> $((total - bad)) certificates"
    fi
  done < "$work/files"
  # Dangling links in the trust directories (hash links and ca-cert-*.pem pointing at removed files).
  for d in $trust_dirs; do
    [ -d "$root/$d" ] || continue
    find "$root/$d" -type l 2>/dev/null | while IFS= read -r l; do
      [ -e "$l" ] || { rm -f "$l"; echo "noemsign-ca: removed dangling link $l"; }
    done
  done
}

guard() {
  pem_files > "$work/files"
  n=0; bad=0
  while IFS= read -r f; do
    split "$f" "$work/s"
    for c in "$work"/s/c*.pem; do
      [ -e "$c" ] || continue
      n=$((n + 1))
      s=$(subject "$c")
      if printf '%s' "$s" | grep -Eiq "$pat"; then bad=$((bad + 1)); echo "noemsign-ca: FAIL $f: $s" >&2; fi
    done
  done < "$work/files"
  for d in $trust_dirs; do
    [ -d "$root/$d" ] || continue
    find "$root/$d" -type f \( -name '*.crt' -o -name '*.der' -o -name '*.cer' \) 2>/dev/null | while IFS= read -r f; do
      grep -q -e '-----BEGIN CERTIFICATE-----' "$f" && continue
      s=$(openssl x509 -inform DER -in "$f" -noout -subject -nameopt RFC2253 2>/dev/null) || continue
      if printf '%s' "$s" | grep -Eiq "$pat"; then echo "noemsign-ca: FAIL $f (DER): $s" >&2; echo x >> "$work/derbad"; fi
    done
  done
  [ ! -s "$work/derbad" ] || bad=$((bad + $(wc -l < "$work/derbad")))
  b="$root/etc/ssl/certs/ca-certificates.crt"
  [ "$root" != / ] || b=/etc/ssl/certs/ca-certificates.crt
  if [ -f "$b" ]; then
    # A scan that found no certificate although the bundle exists means the scanner is broken.
    if [ "$n" -eq 0 ]; then echo "noemsign-ca: FAIL scanner found no PEM file although $b exists" >&2; bad=$((bad + 1)); fi
    m=$(grep -c -e '-----BEGIN CERTIFICATE-----' "$b" || true)
    if [ "$m" -lt 100 ]; then echo "noemsign-ca: FAIL $b has only $m certificates" >&2; bad=$((bad + 1)); fi
  fi
  find "$root" -xdev \( -path "${root%/}/proc" -o -path "${root%/}/sys" -o -path "${root%/}/dev" \) -prune \
    -o -path "$work" -prune -o -type f -exec grep -ail 'emSign' {} + 2>/dev/null | while IFS= read -r f; do
    grep -qxF "$f" "$work/files" && continue
    case "$f" in
      */etc/ca-certificates.conf|*/etc/apk/world|*/lib/apk/db/*|*/var/lib/dpkg/*|*/usr/share/doc/*|*/noemsign-ca.sh) continue ;;
    esac
    echo "noemsign-ca: WARN non-PEM file mentions emSign: $f" >&2
  done
  echo "noemsign-ca: guard scanned $n PEM certificates in $(wc -l < "$work/files") files under $root: $bad emSign"
  [ "$bad" -eq 0 ] || { echo "noemsign-ca: guard FAILED" >&2; exit 1; }
}

case "$mode" in
  strip) strip ;;
  guard) guard ;;
  *) echo "usage: noemsign-ca.sh strip|guard ROOT" >&2; exit 2 ;;
esac
