(plugin-firewall-policy)=
# Firewall policy plugin

This plugin adds a vendor-neutral security-policy data model and renders it as device configuration.

`firewall.policy` is a plugin added to *netlab* by a patch; it is not part of stock *netlab*. It was added alongside `nat` and `loadbalancer` for blueprint-import parity.

## Supported Platforms

| Operating system | Schema | Renders config | Template |
| ---------------- | :--: | :--: | --- |
| Fortinet FortiOS |  ✅  |  ✅  | `fortios.j2` |
| Palo Alto PAN-OS |  ✅  |  ✅  | `panos.j2` |
| `none`           |  ✅  |  ❌  | none — the meta device declares nearly every feature so transform tests pass |

A device that is not in this list and carries `firewall.policy.rules` is an error, not a silent skip — see [Validation](#validation).

## Using the Plugin

```yaml
plugin: [ firewall.policy ]

nodes:
  fw:
    device: fortios
    firewall.policy.rules:
    - name: trust-to-untrust
      action: permit
      from_zone: LAN
      to_zone: WAN
      src: [ 10.0.0.0/8 ]
      dst: [ any ]
      services: [ ALL ]
```

## Supported attributes

Node-level:

* `firewall.policy.rules` — a list of rules.
* `firewall.policy.service_objects` — named protocol/port services (`name`, `protocol` tcp|udp, `port`, optional `description`).
* `firewall.policy.service_groups` — named service GROUPS (`name`, `members`). Members are service objects, inline `tcp/PORT` forms, other groups, or predefined platform services. A rule that names a group keeps the group: FortiOS and PAN-OS emit the group object and reference it, the ASA expands the members into one ACE each.
* `firewall.policy.same_security_traffic` — ASA only: `same-security-traffic permit inter-interface`, without which two interfaces at the same security level never exchange traffic.
* `firewall.policy.inspect` — ASA only: protocol inspection engines added to the factory `global_policy` / `inspection_default` class (`icmp`, `icmp-error`). `icmp` makes the return half of ping/traceroute stateful through a lower-security interface; without it time-exceeded and unreachable replies from `outside` are dropped.
* `firewall.policy.address_objects` — named addresses (`name`, `ipv4` as prefix, host or `a-b` range). FortiOS also generates an address object for any src/dst token that is written as a prefix, host or range, because a FortiGate policy cannot take one inline.

Rule attributes:

| Attribute | Type | Required | Values | Default applied by the plugin |
| --- | --- | :--: | --- | --- |
| `name` | str | no | — | `policy_<n>`, 1-based |
| `sequence` | int | no | — | `10`, then `20`, `30`… (+10 per rule) |
| `action` | str | **yes** | `permit`, `deny` | normalized to `permit` if unrecognized |
| `from_zone` | id | **yes** | — | `any` |
| `to_zone` | id | **yes** | — | `any` |
| `src` | list of str | no | — | `[any]` |
| `dst` | list of str | no | — | `[any]` |
| `services` | list of str | no | — | `[any]` |
| `log` | bool | no | — | `True` |
| `nat` | bool | no | — | `False` — see [Interaction with the NAT plugin](#interaction-with-the-nat-plugin) |

An explicit `sequence` wins but does **not** shift the counter for later rules, so mixing explicit and generated sequence numbers can collide. FortiOS keys its policies on `sequence`; PAN-OS does not use it at all.

### Platform differences worth knowing

* **FortiOS** maps `from_zone`/`to_zone` to `srcintf`/`dstintf` — an interface name or a zone object (bind zones with `firewall.zone` and the `firewall.zonebased` plugin). `set schedule "always"` is emitted on every rule. Rule names are stripped of surrounding double quotes. Objects are rendered before the policy table: `config firewall address` (address objects + inline prefixes), `config firewall service custom` (service objects + inline `tcp/PORT`), `config firewall service group`. Predefined FortiOS services (`HTTP`, `HTTPS`, `SSH`, `DNS`, `LDAPS`, ...) may be named directly: the device declares `features.firewall.policy_named_services`, which exempts bare names from the resolvability check below. `any`/`all` become the built-ins `all` / `ALL`.
* **PAN-OS** keys rules by `name`, so ordering is insertion order, not `sequence`. `application any` is emitted on every rule; there is no `application` attribute. Rule names are **not** quote-stripped, so a name containing `"` produces malformed configuration.
* On PAN-OS the stock `initial/panos.j2` template reads `firewall.policy.rules` directly and auto-creates a `set zone "<name>" network layer3` for every non-`any` zone referenced. That is a hard dependency of the initial config on this plugin's data.

## Validation

Only two conditions are reported, both in `post_transform`, and both are skipped entirely when a node has no rules:

* `Node <name>(<device>) does not support firewall.policy rules` — the device does not declare `features.firewall.policy`. The whole node is skipped.
* `Node <name>(<device>) cannot resolve service '<svc>'` — on a platform without App-ID a service must be `any`, a bare protocol, an inline `tcp|udp/PORT`, a `service_objects` name, a `service_groups` name whose members all resolve, or (on `policy_named_services` platforms) any name. The ASA builds its ACE from the service, so an unresolvable name would render nothing.
* `Node <name> firewall.policy.rules[<idx>] must be a dictionary` — that one rule is skipped; the rest of the node still renders.

There is deliberately **no** duplicate-name check, duplicate-sequence check, zone-existence check, or address/service object validation. An unrecognized `action` is coerced to `permit` rather than rejected — the schema's `valid_values` is what actually rejects a bad action, earlier.

## Interaction with the NAT plugin

A rule carrying `nat: true` is promoted into `node.nat.rules` by the [`nat` plugin](nat.md), which renders the translation. The two plugins are otherwise independent: `nat` no longer *requires* `firewall.policy`, so `plugin: [ nat ]` works on its own.

Promoted rules are **appended** to any NAT rules the author wrote, not substituted for them. They also skip the standalone NAT rule schema — a promoted rule needs no `type` and no `translated_*` attribute, because the platform's policy template renders it.

On FortiOS this is how source NAT is expressed at all: `nat/fortios.j2` re-enters `config firewall policy` and adds `set nat enable` to the policy IDs this plugin created, matching them by `sequence`. That is the platform's model — FortiOS NAT only takes effect where a matching firewall policy exists.
