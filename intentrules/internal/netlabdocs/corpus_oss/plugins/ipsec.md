(plugin-ipsec)=
# IPsec plugin

This plugin turns `type: tunnel` links into vendor-configurable IPsec tunnels.

The plugin uses the tunnel link for overlay addressing and interface creation, and adds IPsec-specific underlay, peer, and crypto data on top of that model.

The current generic model is built around route-based tunnels. Policy-based data can be carried in the model with `mode: policy-based` and `selectors`, but full deployment support depends on the target platform.

## Supported Platforms

| Operating system    | IPsec tunnel deployment |
| ------------------- | :--: |
| Cisco IOS / CSR     |  ✅  |
| Palo Alto PAN-OS    |  ✅  |

## Using the Plugin

* Add `plugin: [ ipsec ]` to the lab topology.
* Use `type: tunnel` on links that should become IPsec tunnels.
* Put shared crypto settings under the link-level `ipsec` attribute.
* Put per-endpoint underlay data under the interface-level `ipsec` attribute.

## Supported attributes

Link-level attributes:
* `ipsec.preshared_key` (string, mandatory) -- shared secret used by both tunnel endpoints.
* `ipsec.mode` (`route-based` or `policy-based`) -- IPsec service model. The default is `route-based`.
* `ipsec.name` (string) -- optional vendor tunnel object name.
* `ipsec.peer_type` (`ip`, `dynamic`, or `fqdn`) -- default peer type for all endpoints on the link.
* `ipsec.selectors` (list) -- policy-based traffic selectors, each entry containing `local` and `remote` prefixes.
* `ipsec.ike` -- IKE settings including `version`, `crypto_profile`, `authentication`, `encryption`, `dh_group`, lifetimes, and `enable_nat_traversal`.
* `ipsec.esp` -- IPsec ESP settings including `crypto_profile`, `authentication`, `encryption`, `dh_group`, and lifetimes.

Interface-level attributes:
* `ipsec.local_interface` (string) -- underlay interface used to terminate the tunnel.
* `ipsec.local_address` (string) -- local underlay source address when the platform needs an address instead of an interface.
* `ipsec.endpoint` (string) -- local underlay address or FQDN. The remote endpoint can use this value automatically.
* `ipsec.peer` (string) -- explicit remote peer override for the local endpoint.
* `ipsec.peer_type` (`ip`, `dynamic`, or `fqdn`) -- local peer type override.
* `ipsec.selectors` (list) -- endpoint-specific selector override or refinement.
* `ipsec.local_id_type` / `ipsec.local_id_value` -- local IKE identity.
* `ipsec.peer_id_type` / `ipsec.peer_id_value` -- remote peer IKE identity.

## Example

```yaml
plugin: [ ipsec ]

provider: clab
defaults.device: csr

nodes: [ r1, r2 ]

links:
- r1:
    ipv4: 192.0.2.1/30
  r2:
    ipv4: 192.0.2.2/30
- r1:
    ipsec:
      local_interface: GigabitEthernet2
      endpoint: 192.0.2.1
  r2:
    ipsec:
      local_interface: GigabitEthernet2
      endpoint: 192.0.2.2
  type: tunnel
  ipsec:
    mode: route-based
    preshared_key: netlab-secret
    ike:
      version: ikev2
    esp:
      dh_group: group14
```
