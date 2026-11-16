# Leaf-spine fabric with an EVPN/VXLAN overlay (fabric plugin)

A known-good starting point: netlab's `fabric` plugin generates the leaf-spine, OSPF is the underlay, iBGP EVPN with the
spines as route reflectors is the overlay, and one VLAN is stretched over VXLAN with an IRB gateway on each leaf. `h1` on `L1` reaches `h2` on `L2`
in the same subnet across the fabric. The `validate:` block proves it with a ping between the hosts.

| | Default |
|---|---|
| Spines / leaves | 2 / 2 (`S1`, `S2`, `L1`, `L2`) |
| Underlay | OSPF on every leaf and spine, /31 links from the `fabric` pool |
| Overlay | iBGP EVPN, AS 65000, spines reflect |
| Data plane | VLAN `red` (id 1000, VNI 101000), irb mode (an SVI per leaf), `ospf: false`, on the leaves |
| Device | `defaults.device: eos`, the only vendor-specific line |

## Why it is written this way

Each of these is a way a first attempt compiles and then does not work, or does not compile with a message that hides the cause.

- **`plugin: [ fabric ]` is required.** The `fabric:` block is read only when the plugin is listed. Without it netlab reports
  the `leafs` and `spines` groups as having "no direct or indirect members" (the plugin would have created them).
- **Every role lists its full modules, the spines included.** `ospf` on the spines is what lets the iBGP/EVPN sessions come up;
  leave it off and the compile succeeds with a warning and no session ever forms. There is no topology-level `module:` list on
  purpose: a node's own list replaces it and a group list may not inherit it, so naming the full set per role behaves the same
  in Skyforge and in upstream netlab.
- **Every VLAN carries `ospf: false`.** The VLAN is in irb mode (each leaf gets an SVI; the collector needs that to see the hosts attach),
  and in irb mode netlab runs OSPF on the SVI. The leaves then form OSPF adjacencies across their own VXLAN tunnel, learn each
  other's VTEP loopback through it (recursive routing), the tunnel black-holes and the adjacency drops, about every 30-40 seconds:
  hosts are reachable only some of the time. In the live proof the same fabric without the line lost every probe in 77 of 128
  attempts and with it lost none of 264. Alternatives: `ospf.passive: true` on the VLAN, or `mode: bridge` when the fabric needs no
  gateway (no SVI, so no OSPF; hosts then need a gateway elsewhere). `scripts/check-netlab-findings.sh` flags a skeleton that
  loses the line (finding `evpn-svi-ospf-overlay`).
- **The link pool is declared.** `fabric.link.pool` names a pool that exists under `addressing:`.
- **Hosts attach to the generated names** (`L1`, `L2`). They are nodes with `device: linux`; netlab has no `hosts:` section.

## Scale it

Change the two counts; nothing else needs to move.

```yaml
fabric:
  spines: 4
  leafs: 16
```

- **Names.** Leaves are `L1..Ln`, spines `S1..Sn`. Attach more hosts with more `links:` entries and `vlan.access: red`
  (add them to the `hosts` group so they get `device: linux`).
- **Links.** The fabric has `spines x leafs` links plus your host links, so the pool needs that many /31s (a /16 holds 32768).
  Compiled counts: 2x2 gives 6 nodes and 6 links with the two hosts; 4x16 gives 22 nodes and 66 links; 8x32 gives 42 nodes and
  258 links. All compile with no warnings (`scripts/check-netlab-findings.sh` checks 2x2 and 4x16 on every run).
- **Addressing.** Loopbacks and the host VLAN come from netlab's default pools; the fabric links come from `fabric`. To pin
  loopback addresses, add `addressing.loopback`.
- **VLANs and VNIs.** Add entries under `vlans:` with their own `id`, `vni` and `ospf: false`; give a leaf's host link `vlan.access: <name>`.
  A VLAN is stretched to every leaf that has a link in it, and the VNI is the same fabric-wide.
- **Capacity.** Every node is a pod or VM: 4x16 is 20 network devices. Check `kubectl describe node` CPU requests before
  launching anything large (see the repo `CLAUDE.md`).
- **A different NOS.** Change `defaults.device`. It must support `evpn` and `vxlan` in netlab and the Skyforge `services`
  module under KNE; eos is the boot-proven choice. (`frr` fails the compile today for the `services` module.)

## What a compile does not prove

The compile is checked on every change; a boot is not. After a boot: `show ip ospf neighbor` on a spine, `show bgp evpn summary`
on a leaf (the spine sessions Established), then the lab's own `validate:` ping.
