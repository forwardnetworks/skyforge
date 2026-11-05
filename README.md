# Skyforge

The patches and netlab topologies we run in Skyforge, our network lab platform on Kubernetes. Shared as is: this is our setup, not a product.

We run it on Talos Linux with Cilium, KubeVirt and Multus. netlab compiles each lab, KNE deploys it, and meshnet wires the links (VXLAN across nodes).

## Contents

- `patches/<project>/`: patch series on pinned upstream versions. See [UPSTREAM.md](UPSTREAM.md).
- `components/blueprints/netlab/`: [ipspace/netlab-examples](https://github.com/ipspace/netlab-examples) adapted to run on KNE.
- `scripts/apply-patches.sh`: clones a pinned upstream and applies its series.

## Apply

```bash
scripts/apply-patches.sh netlab        # or: kne, koko, meshnet-cni, arista-ceoslab-operator
git apply /path/to/patches/nokia-grpc/*.patch   # Ansible collections: run from the collection root
                                                # catalystwan-sdk: run from site-packages (paths start catalystwan/)
```

Patches marked SKYFORGE-SPECIFIC are our platform's behaviour, not upstream candidates. Image registries are placeholders (`registry.example.com`).

## Licence

Docs and scripts: Apache-2.0 ([LICENSE](LICENSE)). Patches: each upstream's licence. Blueprints: ipSpace's licence ([LICENSE.md](components/blueprints/netlab/LICENSE.md)), which forbids offering them as a hosted or SaaS service. See [NOTICE](NOTICE).
