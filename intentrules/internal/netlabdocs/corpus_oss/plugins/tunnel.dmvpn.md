(plugin-tunnel-dmvpn)=
# DMVPN (mGRE + NHRP) Tunnels

The **tunnel.dmvpn** plugin builds a multipoint GRE overlay with NHRP: one or
more hubs act as next-hop servers, spokes register with them, and a routing
protocol (typically EIGRP) runs over the single tunnel interface. There is no
`tunnel destination` -- NHRP resolves it. Cisco IOS/IOL/IOS-XE only.

## Link attributes

| Attribute | Type | Meaning |
| --- | --- | --- |
| `tunnel.mode` | `dmvpn` | Required. |
| `tunnel.hub` | str or list | Node name(s) acting as NHRP next-hop server. Every other node is a spoke. |
| `tunnel.nhrp_id` | int | NHRP network-id (default: the link index). |
| `tunnel.password` | str | NHRP authentication string. |
| `tunnel.registration_timeout` | int | Spoke `ip nhrp registration timeout N` (seconds): how soon a spoke re-registers after a hub flap. IOS defaults to holdtime/3 (up to 40 minutes); a lab that wants fast reconvergence sets it low. Hubs never carry it. |
| `tunnel.source` | dict | Where the tunnel is sourced: `{ link: { role: <role> } }`, `{ link: { name: <name> } }`, `{ ifindex: N }` or `{ type: loopback }`. |
| `tunnel.af` | `ipv4` / `ipv6` | Address family (default ipv4). |
| `tunnel.vrf` | str | Transport VRF. |

The hub renders `ip nhrp map multicast dynamic` (and `no ip split-horizon
eigrp <as>` when EIGRP runs on the tunnel, so spokes see each other's routes);
spokes render `ip nhrp map <hub-overlay> <hub-nbma>`, `ip nhrp map multicast
<hub-nbma>` and `ip nhrp nhs <hub-overlay>` for every hub.
