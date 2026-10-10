# Upstream versions and patch series

Every series applies to a pinned upstream commit or release. The koko, meshnet-cni and arista-ceoslab-operator series apply to a pinned upstream commit: `scripts/apply-patches.sh <component>` clones it and applies the series in order. The Ansible and Python series apply to an installed package tree.

| Component | Upstream | Pinned to | Patches | What the series is for |
|---|---|---|---|---|
| koko | https://github.com/redhat-nfvpe/koko | `a18aa44814eabb542226e11c5568e8433810e4ab` (master) | 15 | Link creation that is idempotent and collision-safe: stale-link cleanup, per-endpoint locking, explicit veth MTU, namespace-scoped VXLAN conflict search, adopting a late wire into a KubeVirt bridge, a read-only wire inspection call, and not binding a cross-node VXLAN to the underlay device. |
| meshnet-cni | https://github.com/networkop/meshnet-cni | `d7c306c635cf73a44833e3ebdefb8b163a9aa784` (master) | 24 | Making link wiring reliable at scale: bounded RPCs and client budgets, deterministic VNI and underlay source selection, KubeVirt secondary-network support, churn damping and locking, uniform fabric MTU with fail-closed underlay validation, a reconciler for wires no CNI event will repair, and cross-node VXLAN attribute comparison. Depends on the koko series. |
| arista-ceoslab-operator | https://github.com/aristanetworks/arista-ceoslab-operator | `v2.1.2` = `72c8685ee56c26087211846a5262d2954f543b59` | 19 | cEOS operator hardening: startup persistence, management routing and return routes, bounded and atomic bring-up, data-plane permits for the container's life, endpoint locking, exact declarative placement, and two host-isolation fixes (a sysctl leak and a `/sys/block` wedge). Later patches in the series edit the same controller file and must be applied in order. |
| catalystwan (Ansible collection) | https://github.com/cisco-en-programmability/ansible-collection-catalystwan | collection `0.3.4` | 5 | Workarounds for Cisco SD-WAN (vManage) behaviour. Applies to the installed collection tree. |
| catalystwan (Python SDK) | https://github.com/cisco-en-programmability/catalystwan | `catalystwan==0.41.5.dev2` | 1 | One model fix for a field vManage returns as a number. Applies to the installed package. |
| nokia.grpc (Ansible collection) | https://github.com/nokia/ansible-networking-collections | collection `1.0.2` | 1 | A fix for a helper that mutated a dict while iterating it. Applies to the installed collection tree. |

## Coming

The netlab and KNE series are being reviewed for release and are not published yet.

These series are small, reviewable and meant to go upstream where the upstream project wants them. None has been accepted upstream yet.
