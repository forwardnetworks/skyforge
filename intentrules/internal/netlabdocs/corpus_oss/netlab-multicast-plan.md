# Netlab Multicast Plan

## Summary

Add IPv4 multicast support to the vendored netlab tree, scoped to Cisco IOL and Arista EOS. The goal is configuration parity for these multicast features: PIM, static RP, IGMP receiver joins, VRF multicast, static multicast RPF routes, and EOS bidirectional PIM.

## Implementation

- Add a `multicast` module with node, VRF, and interface attributes for PIM mode, static RP entries, static multicast routes, and IGMP options.
- Enable the module only on `iol` and `eos`; explicitly keep `ioll2` unsupported.
- Render IOS/IOL commands with `ip multicast-routing`, `ip pim rp-address`, interface `ip pim ...-mode`, IGMP options, and `ip mroute`.
- Render EOS commands with `router multicast`, `router pim sparse-mode` or `router pim bidirectional`, interface PIM/IGMP commands, and static mroutes.
- Validate multicast group prefixes, non-multicast RPF source prefixes, unsupported PIM modes, unsupported VRF multicast, and unsupported bidir use.

## Tests

- Add focused pytest coverage for IOL transform and rendered config.
- Add EOS rendered-config coverage for VRF multicast and bidirectional PIM.
- Run targeted multicast tests plus the documented transform/error test slice when a Python 3.10+ environment with test dependencies is available.

## Out Of Scope

- IPv6 PIM and MLD.
- MSDP.
- NX-OS TRM or MVPN.
- IGMP snooping-only Layer 2 behavior.
- Live multicast traffic validation.
