# Skyforge

The patches and netlab topologies we run in Skyforge, our network lab platform on Kubernetes. Shared as is: this is our setup, not a product.

We run it on Talos Linux with Cilium, KubeVirt and Multus. netlab compiles each lab, KNE deploys it, and meshnet wires the links (VXLAN across nodes).

## What is and is not here

Here: the patch series, the netlab blueprints (8 of them carry `validate:` checks, which `netlab validate` runs against a booted lab, for example an end-to-end ping across an EVPN/VXLAN overlay), and the install guide.

Not here: the Skyforge control plane and portal (the web service that deploys labs, runs change checks and pushes changes). They are not published and there is no date.

## Contents

- `patches/<project>/`: patch series on pinned upstream versions. See [UPSTREAM.md](UPSTREAM.md).
- `components/blueprints/netlab/`: [ipspace/netlab-examples](https://github.com/ipspace/netlab-examples) adapted to run on KNE.
- `scripts/apply-patches.sh`: clones a pinned upstream and applies its series.

To deploy the stack from scratch (Talos, add-ons, KNE, netlab), see [INSTALL.md](INSTALL.md).

## Apply

```bash
scripts/apply-patches.sh netlab        # or: kne, koko, meshnet-cni, arista-ceoslab-operator
git apply /path/to/patches/nokia-grpc/*.patch   # Ansible collections: run from the collection root
                                                # catalystwan-sdk: run from site-packages (paths start catalystwan/)
```

Patches marked SKYFORGE-SPECIFIC are our platform's behaviour, not upstream candidates. Image registries are placeholders (`registry.example.com`).

## Intent rules

`intentrules/` is a small standalone Go module: a design-time rules layer for netlab topologies (Infrahub-format schema plus a sidecar of
constraints). See [intentrules/README.md](intentrules/README.md).

## Licence

The docs and scripts we wrote here, and the `intentrules/` module: [Fair Core License 1.0, Apache-2.0 future licence](LICENSE) (FCL-1.0-ALv2). It is source-available, not open source: you can read, run, modify and redistribute it for any purpose except a competing commercial product or service, and each release converts to Apache-2.0 two years after it is published. Commits up to `aa25fed` were published under Apache-2.0, and commits from `4c6e1b6` up to `3775ce0` under the Functional Source License 1.1 (FSL-1.1-ALv2); each keeps the licence it was published with. Patches: each upstream's licence. Blueprints: ipSpace's licence ([LICENSE.md](components/blueprints/netlab/LICENSE.md)), which forbids offering them as a hosted or SaaS service. See [NOTICE](NOTICE).

Licensing and commercial questions: craigjohnson@forwardnetworks.com
