#!/usr/bin/env bash
# Clone a pinned upstream and apply one patch series to it.
#   scripts/apply-patches.sh <component> [workdir]
# Components: netlab kne meshnet-cni koko arista-ceoslab-operator
# The Ansible/Python series (catalystwan, catalystwan-sdk, nokia-grpc) apply to an installed package tree: see README.md ("Apply").
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
comp="${1:?usage: apply-patches.sh <component> [workdir]}"
work="${2:-$here/.work}"
case "$comp" in
  netlab)                 url=https://github.com/ipspace/netlab.git;                       rev=ff52463be991e5e0c17ea396254625c25fef9aa6 ;;
  kne)                    url=https://github.com/openconfig/kne.git;                       rev=3fe74f99999ff8dbb685f45a6c18757ee0d2cf99 ;;
  meshnet-cni)            url=https://github.com/networkop/meshnet-cni.git;                rev=d7c306c635cf73a44833e3ebdefb8b163a9aa784 ;;
  koko)                   url=https://github.com/redhat-nfvpe/koko.git;                    rev=a18aa44814eabb542226e11c5568e8433810e4ab ;;
  arista-ceoslab-operator) url=https://github.com/aristanetworks/arista-ceoslab-operator.git; rev=72c8685ee56c26087211846a5262d2954f543b59 ;;
  *) echo "unknown component: $comp (Ansible/Python series: see README.md, "Apply")" >&2; exit 2 ;;
esac
dest="$work/$comp"
mkdir -p "$work"
if [ ! -d "$dest/.git" ]; then git init -q "$dest"; git -C "$dest" remote add origin "$url"; fi
git -C "$dest" fetch -q --depth 1 origin "$rev"
git -C "$dest" checkout -q --detach FETCH_HEAD
git -C "$dest" -c user.name=patches -c user.email=patches@example.invalid reset -q --hard "$rev" 2>/dev/null || true
n=0
ident=(-c user.name=patches -c user.email=patches@example.invalid)
for p in $(ls "$here/patches/$comp"/*.patch | sort); do
  if head -1 "$p" | grep -Eq '^From [0-9a-f]{40} '; then
    # a git format-patch mail: keep its author and message
    git -C "$dest" "${ident[@]}" am -q --3way "$p" || { echo "FAILED: $p" >&2; exit 1; }
  else
    # a plain diff with a descriptive first line
    git -C "$dest" apply --index --3way "$p" || { echo "FAILED: $p" >&2; exit 1; }
    git -C "$dest" "${ident[@]}" commit -q -m "$(head -1 "$p")"
  fi
  n=$((n+1))
done
echo "$comp: applied $n patches on $rev in $dest"
