# Load Balancer Plugin

The **loadbalancer** plugin preserves and renders load-balancing metadata discovered by an importer.

```yaml
plugin: [ loadbalancer ]
```

## Data Model

The plugin expects node-level `load_balancing` data:

```yaml
nodes:
  lb1:
    load_balancing:
      virtual_servers: []
      service_groups: []
      servers: []
```

Malformed structures are reported as transformation errors.

### Structured model and VIP advertisement

The structured `loadbalancer` node attribute (`monitors`, `pools`,
`virtual_servers`) is what the device templates render. A virtual server can
set `advertise: true` to inject its VIP into the node's routing protocols:

```yaml
nodes:
  lb1:
    device: a10
    module: [ ospf ]
    loadbalancer:
      pools:
      - name: web
        members: [ { name: s1, address: 10.0.0.11, port: 80 } ]
      virtual_servers:
      - { name: vip1, vip: 203.0.113.10, port: 80, pool: web, advertise: true }
```

On A10 ACOS this renders `redistribution-flagged` on that virtual server and
`redistribute vip only-flagged` under `router ospf`; virtual servers without
`advertise` stay local.

### Health monitors

`loadbalancer.monitors` declares named probes and three attributes bind them,
one per level a load balancer checks health at:

| Attribute | Checks | A10 ACOS rendering |
|---|---|---|
| `pools[].monitor` | the pool / service group | `slb service-group <p>` → `health-check <m>` |
| `members[].monitor` (default `pools[].member_monitor`) | the member service (ip:port) | `slb server <s>` → `port <n> <proto>` → `health-check <m>` |
| `members[].server_monitor` (default `pools[].server_monitor`) | the member host | `slb server <s>` → `health-check <m>` |

Each binding takes a `monitors[].name`, `ping` (the built-in ICMP probe, never
declared), `none` (health checking disabled: `health-check-disable` on ACOS) or
`default` (no binding -- the device default applies). A reference to an
undeclared monitor is an error.

Monitor attributes: `type` (`icmp` -- `ping` is accepted and normalized to
`icmp` -- `tcp` or `http`), `port` (required for `tcp`), `url` (HTTP request
path, default `/`), `interval` and `timeout` (seconds, rendered only when set;
`timeout` must not exceed `interval`). `send`/`receive` are carried but not
rendered on A10.

```yaml
loadbalancer:
  monitors:
  - { name: webfarm_check, type: http, port: 80, url: /index.php }
  - { name: hm_vip_80, type: icmp, interval: 10 }
  pools:
  - name: web
    monitor: ping              # service-group level
    server_monitor: ping       # every member host
    member_monitor: ping       # every member service
    members:
    - { name: s1, address: 10.0.0.11, port: 0, server_monitor: none }
```

Without a binding ACOS falls back to its defaults -- an ICMP check on the
server and an L4 TCP handshake on each server port -- so a pool whose members
run no listener on the service port reads Down although they answer ping.
Only the A10 template renders monitors today; the F5 template renders the
imported `load_balancing` model and ignores them.

## Capability Contract

Rendering is enabled by device capability:

```yaml
features:
  loadbalancer: {}
```

Behavior:
- If `features.loadbalancer` is present (or any non-`False` value), the plugin schedules extra config rendering for that device (`api.node_config(node, "loadbalancer")`).
- If capability is absent, the plugin keeps warning behavior:
  - `Node <name>(<device>) has load-balancing metadata; rendering of these features is not implemented yet`

This keeps onboarding generic: new load-balancer platforms do not require plugin code changes.

## Onboarding a New LB Platform

To onboard a new load-balancer device:

1. Add/extend device definition in `netsim/devices/<device>.yml`
   - include `features.loadbalancer: {}`
2. Add template in `netsim/extra/loadbalancer/<device>.j2`
3. Ensure the device has a delivery lane: an Ansible deploy path (`deploy-config/<device>.yml`) or
   `netlab_config_mode: netmiko`. Without one netlab renders the template and silently skips the push
   (A10 is delivered over netmiko, device type `a10`, saved with `netmiko_save_command`)
4. Add/update tests:
   - positive rendering test (`netlab create ...` without warning)
   - negative/unsupported test (warning still emitted when capability is missing)

## Current Implementation

- Implemented template: `netsim/extra/loadbalancer/f5.j2`
- Existing test coverage includes:
  - rendering test for supported F5
  - warning-preservation test for unsupported devices
