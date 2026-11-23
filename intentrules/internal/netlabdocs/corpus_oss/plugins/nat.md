(plugin-nat)=
# NAT plugin

This plugin adds a generic NAT data model to supported firewall and router operating systems, and renders vendor configuration for the platforms marked below.

`nat` is a plugin added to *netlab* by a patch; it is not part of stock *netlab*. It is maintained alongside `firewall.policy` and `loadbalancer`.

## Supported Platforms

The earlier version of this plugin carried a schema only and rendered no device configuration. That is no longer uniformly true, and the distinction matters: a platform with a schema but no template will accept `plugin: [ nat ]`, validate your rules, report the feature as supported, and emit **no NAT configuration at all**.

| Operating system    | NAT schema | Renders config | Template |
| ------------------- | :--: | :--: | --- |
| Arista EOS          |  ✅  |  ✅  | `eos.j2` — interface-based |
| Cisco IOS / IOS-XE  |  ✅  |  ✅  | `ios.j2` — inside/outside + global statement |
| Cisco IOL           |  ✅  |  ✅  | `iol.j2` — includes `ios.j2` |
| Palo Alto PAN-OS    |  ✅  |  ✅  | `panos.j2` — zone-based, own NAT rulebase |
| Fortinet FortiOS    |  ✅  |  ⚠️  | `fortios.j2` — see the FortiOS exception |
| Juniper vSRX        |  ✅  |  ✅  | `vsrx.j2` — source NAT only, one `security nat source rule-set` per rule (interface or pool); from/to zones from the rule's `inside`/`outside` zone names or the `firewall.zone` of the interfaces carrying `nat.role` |
| Cisco ASA / ASAv    |  ✅  |  ❌  | **none — schema only** |

**Every NAT template lives in this plugin.** PAN-OS NAT is its own rulebase (`rulebase nat rules`), completely separate from `rulebase security rules`, so it belongs here like every other platform. It is easy to assume `firewall.policy/panos.j2` covers it — it does not, and never did: that template emits SECURITY rules only. Before `panos.j2` existed, PAN-OS was declared `features.nat: True` and rendered nothing at all.

**The FortiOS exception.** In FortiOS, source NAT genuinely *is* a flag on a firewall policy (`set nat enable`), so `fortios.j2` re-opens `config firewall policy` and sets it per rule. That is the platform's model, not a layering choice — but it does mean FortiOS NAT only takes effect where a corresponding firewall policy exists. It is marked ⚠️ rather than ✅ for that reason.

**ASAv** is declared and unimplemented. Do not read the schema column as "this works".

## Using the Plugin

* Add `plugin: [ nat ]` to the lab topology. **No other plugin is required.**
* Mark NAT-facing interfaces with `nat.role: inside` or `nat.role: outside`.
* Add node-level NAT rules under `nat.rules`.

The plugin used to declare `_requires: [ firewall.policy ]`, which *netlab* enforces as a hard dependency — `plugin: [ nat ]` alone was rejected at compile time and every pure-NAT topology had to load a firewall plugin it never used. You can translate addresses without having any policy at all, so that dependency is gone. `_execute_after` preserves the ordering when both plugins *are* loaded, so policy-derived NAT rules are still promoted correctly.

### Interface roles are not optional in the same way on every platform

* **EOS** applies the translation on the **outside** interface. An outside role is enough.
* **IOS / IOL** need **both** `inside` and `outside`: the inside/outside markings and the global `ip nat inside source` statement are three separate pieces of configuration, and a node with only one role cannot translate anything.

Both templates say so in the rendered configuration rather than emitting a partial config: a node with rules but without the roles it needs renders a `! NAT NOT APPLIED` comment explaining what is missing. A lab that comes up green while silently doing no translation is the worst possible outcome, so the templates are deliberately loud.

## Supported attributes

Interface-level attributes:
* `nat.role` (`inside` or `outside`) -- classifies the interface for NAT policy.

Node-level attributes:
* `nat.rules` -- list of NAT rules.

Rule attributes:
* `name` -- rule name. Used to name the generated ACL on IOS/EOS (`NAT-<NAME>`).
* `type` (`source`, `destination`, or `static`) -- NAT rule type.
* `description` -- optional description.
* `enabled` -- optional enable flag, defaults to `True`. Only an explicit `enabled: false` suppresses a rule.
* `inside` -- list of source-side zones or interfaces.
* `outside` -- list of destination-side zones or interfaces.
* `source` -- source match prefixes or objects.
* `destination` -- destination match prefixes or objects.
* `service` -- service match values.
* `translated_source` -- translated source value for source/static NAT. On EOS and IOS, omitting it (or setting it to `interface`) means PAT/overload onto the outside interface address.
* `translated_destination` -- translated destination value for destination/static NAT.
* `translated_service` -- translated service value.

The plugin normalizes scalar match values into lists and records discovered interface roles and zones under `node.nat._roles` and `node.nat._zones`.
If another plugin provides interface zone metadata, the NAT plugin records those zones under `node.nat._zones`. Otherwise only interface roles are collected.

## Example: PAT on an Arista switch

```yaml
plugin: [ nat ]

nodes:
  s2:
    device: eos
    nat.rules:
    - name: outbound
      type: source
      source: [ 10.0.0.0/8 ]
      translated_source: interface

links:
- s2:
    ipv4: 10.0.0.1/24
    nat.role: inside
  h1:
- s2:
    ipv4: 203.0.113.1/30
    nat.role: outside
```

Renders on `s2`:

```
ip access-list standard NAT-OUTBOUND
   permit 10.0.0.0/8
!
interface Ethernet2
   ip nat source dynamic access-list NAT-OUTBOUND overload
!
```

The same topology with `device: iol` renders the IOS form instead — standard ACL with a wildcard mask, `ip nat inside` / `ip nat outside` on the interfaces, and the global statement:

```
ip access-list standard NAT-OUTBOUND
 permit 10.0.0.0 0.255.255.255
!
interface Ethernet0/1
 ip nat inside
!
interface Ethernet0/0
 ip nat outside
!
ip nat inside source list NAT-OUTBOUND interface Ethernet0/0 overload
```

## Example: policy-based NAT on a firewall

```yaml
plugin: [ nat ]

provider: clab

nodes:
  fw:
    device: panos
    nat:
      rules:
      - name: trust_to_untrust
        type: source
        inside: trust
        outside: untrust
        source: 10.0.0.0/24
        translated_source: interface

links:
- fw:
    ipv4: 10.0.0.1/24
    nat.role: inside
- fw:
    ipv4: 203.0.113.1/30
    nat.role: outside
```
