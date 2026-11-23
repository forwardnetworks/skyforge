(module-sdwan)=
# SD-WAN module

The SD-WAN module describes an SD-WAN fabric — roles, site identity, transport colours and service VPNs — and exports it as inventory for downstream consumers.

`sdwan` is a module added to *netlab* by a patch; it is not part of stock *netlab*.

## It configures nothing, on purpose

The module appends `sdwan` to every node's `skip_config`, so *netlab* renders **no device configuration** for it and no `.j2` templates exist for any supported platform. That is the design, not a gap: SD-WAN onboarding is performed by the vendor's own controllers, and a second controller writing device config would fight them.

What the module produces instead is **metadata** — `topology.sdwan.inventory`, plus (under `sdwan.system`) lifecycle phases and a capacity reservation, consumed by the KNE provider.

So: use this module to *model* a fabric and hand a correct inventory to whatever onboards it. Do not expect it to configure a router.

```{note}
That is no longer the whole story. The module still renders no
templates of its own, but it now PUSHES OVERLAY FACTS onto each node --
`sdwan.vbond_address`, `organization`, `site_id`, `system_ip`, `is_validator`
and `transport_interfaces` -- and the DEVICE day-0 templates
(`initial/cisco-v*.j2`) consume them. So a Viptela node does come up knowing
its identity and validator address; what it still does not get from netlab is
certificates, controller onboarding or policy, which remain the vendor
controller's job.
```

## Supported Platforms

Device support is the `features.sdwan.types` list on each device; there is no `supported_on` key.

The node attribute follows containerlab, which scopes `type` to `kind`: on the `cisco_sdwan` appliances the type names which control-plane node this is, and on `cat8000v` `type: controller` means "boot SD-WAN managed". **There is no `underlay` type.** A plain transport router is not a member of the module at all -- it carries the tunnels without being part of the fabric -- so the module is declared PER NODE rather than globally.

| Device | Roles |
| --- | --- |
| `cisco_sdwan-manager` | manager |
| `cisco_sdwan-controller` | controller, validator |
| `cisco_sdwan-validator` | validator (declared only -- no image is published, so a topology cannot deploy one; run the validator as a `cisco_sdwan-controller`) |
| `cat8000v` | controller (managed; the fabric role `edge` is derived) |
| `versa-director` | controller |
| `versa-flexvnf` | edge |

```{warning}
`analytics` is a legal value for `sdwan.type` and is accepted by the schema, but **no device declares it**. Any node set to `analytics` fails the type-support check. It is currently unreachable.
```

## Using the module

```yaml
provider: kne
module: [ sdwan ]

sdwan:
  organization: lab
  inventory:
    name: cisco-sdwan

nodes:
  transport:
    device: iol
  vsmart:
    device: cisco_sdwan-controller
  edge1:
    device: cat8000v
    sdwan:
      site_id: 101
      system_ip: 10.255.0.101

links: [ transport-edge1 ]
```

Both the dotted (`sdwan.type: controller`) and nested (`sdwan: {site_id: …}`) forms are valid.

### Declaring a whole fabric with `sdwan.system`

`sdwan.system` expands one declaration into a complete control plane. This topology generates six nodes, one link, four lifecycle phases and a capacity reservation:

```yaml
provider: kne

sdwan:
  system: catalyst
  profile: minimum-lab
  organization: se-lab
  system_ip_pool: 10.255.0.0/24
  control_plane:
    attach: transport
  edges:
    attach: transport
    defaults:
      site_id_start: 101
    members:
      branch1: {}
      branch2: {}
  trust:
    bundle_ref: catalyst-lab-trust

nodes:
  transport:
    device: iol
```

Generated: `sdwan_manager` (cisco_sdwan-manager), `sdwan_controller` (cisco_sdwan-controller), `sdwan_validator` (cisco_sdwan-validator), plus one `cat8000v` per member. `system_ip` offsets are 1/2/3 for the control plane and `site_id_start` upward for edges.

`trust.bundle_ref` names an existing SOPS-managed Kubernetes Secret. It is an **opaque reference** — no key material is stored in the topology or its rendered manifest.

Capacity is reserved as `cpu = 24 + 3×edges`, `memory = (44 + 6×edges) Gi`, `ephemeral-storage = (551 + 50×edges) Gi`. A two-edge lab therefore reserves 30 CPU / 56Gi / 651Gi — plan the substrate before deploying one.

```{warning}
`sdwan.system: versa` is parked: `Versa SD-WAN system is parked: headend licensing entitlement is unresolved`. Only `catalyst` expands today.
```

## Supported attributes

Global: `system` (`catalyst`, `versa`), `profile` (`minimum-lab`), `vendor` (`cisco`/`versa`, default `cisco`), `organization`, `fabric` (bool, default `false`), `system_ip_pool` (prefix), `site_id_pool`, `transport_vpn` (default `0`), `service_vpns`, `colors`, `controllers` (`manager`/`controller`/`validator`), `control_plane.attach`, `edges.*`, `trust.*`, `inventory`.

Node: `type` (`manager`, `controller`, `validator`, `analytics`, `edge`), `site_id`, `system_ip`, `transport` (list of `{interface, color, public_ip}`), `service_vpns`, `service_vpn`.

Link and interface: `role` (`transport`, `service`, `management`), `color`, `vpn`.

### The service side of a WAN edge (`service_vpn`)

An edge that carries only VPN 0 (transport) and VPN 512 (management) is enrolled and routes nothing: its site is unreachable over the overlay. `sdwan.service_vpn: <n>` on an edge puts **every addressed data interface that is neither transport nor management** into service VPN `n`, together with the edge's loopback -- the author never names an interface, netlab already knows which link faces the site. A link can pin a different VPN with `sdwan.vpn` (which implies `sdwan.role: service`). One service VPN per edge.

The site's routing protocol is the edge's own netlab module: put `ospf`, `bgp` (with `bgp.as`) or `eigrp` (with `eigrp.as`) on the edge exactly as on any router and netlab computes areas, neighbours and router-ids as usual. The seam to OMP is derived: an edge with BGP redistributes OMP into BGP and advertises BGP into OMP (its OSPF, if any, stays intra-area); an edge with OSPF alone redistributes OMP into OSPF and advertises `ospf external`; EIGRP redistributes OMP and advertises `eigrp`. Connected and static are always advertised.

**This is day-1, not day-0.** The CVAC bootstrap (`initial/cat8000v-kne.j2`) is untouched: the routing modules are added to the edge's `skip_config` so nothing of this lands in its startup configuration. The intent is rendered into the manager's onboarding document (`sdwan.onboarding.edges[].service`) and delivered by vManage as a per-edge CLI template by `tasks/cisco_sdwan-manager/sdwan_service_vpns.yml`, only for edges the admission lane has PROVEN, with its own per-edge verdict (`PUSHED` / `REFUSED` / `NOT_ATTEMPTED`, read back from the device's running configuration and the controller's OMP table).

```yaml
nodes:
  cdg-vedge01:
    device: cat8000v
    module: [ sdwan, routing, ospf ]
    sdwan: { type: controller, site_id: 100, system_ip: 10.255.0.100, service_vpn: 200 }
```

`site_id_pool` and `edges.defaults.transport_color` are accepted by the schema but never read by the module.

## Validation
