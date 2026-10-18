# Skyforge

Skyforge is a platform for running real network labs on Kubernetes. [netlab](https://github.com/ipspace/netlab) compiles a topology, [KNE](https://github.com/openconfig/kne) turns it into pods and KubeVirt VMs, and [meshnet-cni](https://github.com/networkop/meshnet-cni) wires the links, including across nodes.

This repository is the start of an open-source release. For now it contains one thing: the patch series we carry on top of upstream projects, so that anyone running network labs can read them, apply them, and send the useful ones upstream. The netlab and KNE series follow once they have been reviewed for release.

**Not included yet:** the control plane, the portal, and any product-specific components. Those are not part of this release.

## What is here

| Series | Applies to | Patches |
|---|---|---|
| `patches/koko` | [redhat-nfvpe/koko](https://github.com/redhat-nfvpe/koko) | 15 |
| `patches/meshnet-cni` | [networkop/meshnet-cni](https://github.com/networkop/meshnet-cni) | 24 |
| `patches/arista-ceoslab-operator` | [aristanetworks/arista-ceoslab-operator](https://github.com/aristanetworks/arista-ceoslab-operator) | 19 |
| `patches/catalystwan` | [Cisco SD-WAN Ansible collection](https://github.com/cisco-en-programmability/ansible-collection-catalystwan) 0.3.4 | 5 |
| `patches/catalystwan-sdk` | [catalystwan Python SDK](https://github.com/cisco-en-programmability/catalystwan) 0.41.5.dev2 | 1 |
| `patches/nokia-grpc` | [`nokia.grpc` Ansible collection](https://github.com/nokia/ansible-networking-collections) 1.0.2 | 1 |

**Coming:** netlab and KNE.

[`UPSTREAM.md`](UPSTREAM.md) lists the exact upstream version each series applies to and what the series is for.

## Talks

- [NANOG 98: What Building a Network Emulation Platform Taught Me About Digital Twins](docs/talks/nanog98/README.md): slides and speaker notes.

## Why patches, not forks

Each series is a set of small, reviewable patches on a pinned upstream version. A patch that fixes a general bug is a candidate for upstream. A patch that is specific to how we run labs says so in its header. Nothing here has been accepted upstream yet.

## Apply a series

The koko, meshnet-cni and arista-ceoslab-operator series apply to a pinned upstream commit; the Ansible and Python series apply to an installed package tree.

```bash
scripts/apply-patches.sh koko          # clones the pinned upstream into .work/koko and applies the series
scripts/apply-patches.sh meshnet-cni
```

For an installed collection or package tree, from the root of that tree:

```bash
git apply --check /path/to/skyforge/patches/nokia-grpc/*.patch   # dry run
git apply /path/to/skyforge/patches/nokia-grpc/*.patch
```


## Licensing

The documentation and tooling in this repository (README, `UPSTREAM.md`, `scripts/`) are licensed under the [Apache License 2.0](LICENSE).

Each patch series is a derivative of the project it modifies and stays under that project's license, not Apache-2.0. For example, a patch to netlab is governed by netlab's license. See the upstream repository linked in `UPSTREAM.md` for each project's terms.

## Status

Early. The repository layout, the apply tooling, and which components are published will change.
