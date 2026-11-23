# Netlab SD-WAN Framework Plan

## Purpose

Add SD-WAN as a first-class netlab framework that can describe, validate, render, and test SD-WAN labs without tying the core model to a single external consumer. Cisco SD-WAN and Versa should be the first implementation targets because their controller and edge roles are well understood and useful for collection/modeling workflows.

The first implementation should be metadata-first. It should let users describe an SD-WAN fabric, assign roles, render provider artifacts, and emit a machine-readable inventory. It should not pretend to fully automate vendor onboarding, certificate enrollment, or controller policy workflows before those paths have been validated on real images.

## Design Principles

- Keep netlab device types platform-oriented. A device type should identify the image/runtime shape, while `sdwan.role` identifies whether the node is a manager, controller, validator, edge, or underlay.
- Keep the SD-WAN module vendor-neutral. Cisco SD-WAN and Versa should plug into shared role, site, transport, service VPN, and inventory data structures.
- Keep proprietary image acquisition outside netlab. Netlab can document expected image names and registry placeholders, but should not download licensed images.
- Keep provider plumbing separate from fabric automation. KNE, containerlab, and libvirt need image/runtime contracts; the SD-WAN module needs topology intent and validation.
- Make external collection metadata optional. Netlab should produce a neutral SD-WAN inventory first; downstream orchestration metadata can be derived from that inventory where needed.
- Do not introduce project-specific names, registries, namespaces, secrets, or deployment assumptions into netlab.

## Scope

In scope:

- A new `sdwan` module with global, node, link, and interface attributes.
- Generic SD-WAN roles: `manager`, `controller`, `validator`, `edge`, and `underlay`.
- Generic fabric data: vendor, organization, site IDs, system IPs, transport VPN, service VPNs, transport colors, controller references, and optional external inventory hints.
- Cisco SD-WAN device definitions and role support.
- Versa Director and FlexVNF device definitions and role support.
- KNE provider mappings for the new devices using vrnetlab-style placeholder images.
- Transform, validation, provider-render, and integration fixtures.
- Optional collector inventory output that can be consumed by downstream orchestration or another collection system.

Out of scope for the first pass:

- Native Apple Silicon Cisco SD-WAN appliance support.
- Full Cisco SD-WAN certificate, serial authorization, or vManage template attachment automation.
- Full Versa Director workflow automation.
- Treating IOL as an SD-WAN controller or edge.
- Adding SD-WAN vendors without a clear netlab device/runtime contract.
- Project-specific deployment logic.

## Target Architecture

The framework should have four layers:

1. `sdwan` module data model

   Defines SD-WAN intent, validates role/vendor support, normalizes node metadata, assigns defaults where safe, and builds a neutral inventory.

2. Device definitions

   Declare whether each netlab device can be an SD-WAN manager, controller, validator, edge, or underlay. Device files also carry provider image defaults, Ansible defaults, and feature flags.

3. Provider mappings

   Translate device definitions into runtime artifacts. KNE should know vendor/model/OS/image/service exposure and whether startup config injection is supported.

4. Optional consumers

   Output plugins, provider metadata, or downstream tooling can translate the neutral SD-WAN inventory into a consumer-specific format.

## Neutral SD-WAN Data Model

### Global Attributes

```yaml
module: [ sdwan ]

sdwan:
  vendor: cisco
  organization: lab
  fabric: false
  system_ip_pool: 10.255.0.0/24
  site_id_pool: 100
  transport_vpn: 0
  service_vpns: [10, 20]
  colors: [public-internet, biz-internet]
  controllers:
    manager: vmanage
    controller: vsmart
    validator: vbond
  inventory:
    name: cisco-sdwan
```

Recommended global attributes:

- `vendor`: SD-WAN implementation target. Initial values: `cisco`, `versa`.
- `organization`: Vendor fabric organization/name when relevant.
- `fabric`: When true, require a complete vendor fabric instead of allowing inventory-only partial topologies.
- `system_ip_pool`: Optional pool used to allocate missing node `system_ip` values.
- `site_id_pool`: Optional starting value used to allocate missing edge `site_id` values.
- `transport_vpn`: Vendor-neutral transport segment ID. Cisco default is VPN 0.
- `service_vpns`: List of service segment IDs attached to edge nodes.
- `colors`: Valid transport color names for the topology.
- `controllers`: Named controller references.
- `inventory`: Neutral output controls such as setup name and inclusion rules.

### Node Attributes

```yaml
nodes:
  edge1:
    device: cat8000v
    sdwan:
      role: edge
      site_id: 101
      system_ip: 10.255.0.101
      transport:
        - interface: GigabitEthernet1
          color: public-internet
          vpn: 0
          public_ip: true
      service_vpns:
        10:
          interfaces: [GigabitEthernet2]
```

Recommended node attributes:

- `role`: One of `manager`, `controller`, `validator`, `edge`, or `underlay`.
- `site_id`: Required for edges once configuration generation starts; optional for metadata-only render.
- `system_ip`: Required for Cisco SD-WAN bootstrap; useful inventory metadata for all vendors.
- `transport`: List of SD-WAN transport attachments.
- `service_vpns`: Service segment to interface mapping.
- `controllers`: Optional per-node override for controller references.
- `bootstrap`: Vendor-specific day-0 seed options once validated.
- `inventory`: Optional per-node collection metadata override.

### Link and Interface Attributes

```yaml
links:
- edge1:
    sdwan:
      role: transport
      color: public-internet
  inet:

- edge1:
    sdwan:
      role: service
      vpn: 10
  lan1:
```

Recommended link/interface attributes:

- `role`: `transport`, `service`, or `management`.
- `color`: Transport color for SD-WAN transport links.
- `vpn`: Transport or service segment ID.
- `public_ip`: Whether this transport should be treated as public/NAT-facing.

## Transformation Responsibilities

The `sdwan` module should eventually do the following:

- Normalize `node.sdwan` into a dictionary when the module is enabled.
- Assign default role based on device type only when the role is unambiguous.
- Validate that the device supports the requested role through `features.sdwan.roles`.
- Validate that device vendor and fabric vendor match, except for `underlay`.
- Validate required roles when `sdwan.fabric: true`.
- Validate transport colors against the topology color list.
- Validate that edge nodes have usable site and system identity when bootstrap generation is enabled.
- Copy relevant link/interface SD-WAN metadata into normalized node transport/service structures.
- Build a neutral `topology.sdwan.inventory` structure.
- Let consumer-specific outputs derive their own format from the neutral inventory.

The module should avoid generating vendor configuration until the device templates and image behavior have been validated. In the first pass, it is acceptable for the module to add `sdwan` to `skip_config` for devices that have no SD-WAN template.

## Inventory Contract

The neutral inventory should be the stable internal contract. A suggested shape:

```yaml
sdwan:
  inventory:
    name: cisco-sdwan
    vendor: cisco
    type: cisco_sdwan
    nodes:
      - node: vsmart
        role: controller
        device: cisco-smart
        vendor_os: cisco_sdwan_controller
        address: 192.0.2.11
      - node: edge1
        role: edge
        device: cat8000v
        vendor_os: cisco_ios_xe
        address: 192.0.2.21
        site_id: 101
        system_ip: 10.255.0.101
```

Downstream orchestration can translate the same inventory into its own setup model.

## Cisco SD-WAN Target

### Naming

Use Cisco SD-WAN naming in netlab device names. Avoid `viptela-*` device names. The token `viptela` can still appear where an external tool, Ansible plugin, or collector uses it as an OS/driver identifier.

### Devices

`cat8000v`

- Purpose: Catalyst 8000V used as IOS-XE SD-WAN/cEdge.
- Role support: `edge`, `underlay`.
- Provider image: existing `cat8000v` image contract.
- Collector mapping: `cisco_ios_xe_ssh` for IOS-XE SD-WAN edge collection.
- Bootstrap path: future cEdge template once SD-WAN mode and day-0 behavior are validated.

`cisco-edge`

- Purpose: classic Cisco vEdge Cloud.
- Role support: `edge`.
- KNE model: `vedge`.
- Placeholder image: `vrnetlab/vr-vedge:<version>`.
- Collector mapping: `cisco_sdwan_ssh`.
- Bootstrap path: future vEdge day-0 template if image behavior is predictable.

`cisco-smart`

- Purpose: Cisco SD-WAN Controller, formerly vSmart.
- Role support: `controller`.
- KNE model: `vsmart`.
- Placeholder image: `vrnetlab/vr-vsmart:<version>`.
- Collector mapping: `cisco_sdwan_vsmart_ssh`.
- Bootstrap path: metadata first; real controller bootstrap requires certificate and organization workflow validation.

`cisco_sdwan-validator`

- Purpose: Cisco SD-WAN Validator, formerly vBond.
- Role support: `validator`.
- KNE model: `vbond`.
- Placeholder image: `vrnetlab/vr-vbond:<version>`.
- Collector mapping: none in the first pass.
- Bootstrap path: fabric helper only until validated.

`cisco-manage`

- Purpose: Cisco SD-WAN Manager, formerly vManage.
- Role support: `manager`.
- KNE model: `vmanage`.
- Placeholder image: `vrnetlab/vr-vmanage:<version>`.
- Collector mapping: none in the first pass unless a validated manager API workflow is added.
- Bootstrap path: fabric helper only until validated.

### Cisco Validation Rules

- `sdwan.fabric: true` requires manager, controller, validator, and at least one edge.
- `sdwan.fabric: false` may allow partial topologies for transform, inventory, or externally prebuilt fabrics.
- `cat8000v` can be an edge or underlay, but only becomes SD-WAN when `sdwan.role: edge`.
- IOL, EOS, and `cat8000v` in underlay mode can be underlay only; keep Linux out of the SD-WAN underlay contract because it is not a useful downstream modeling target.
- Cisco SD-WAN VM appliances require an x86-capable provider/runtime.

### Cisco Bootstrap Phases

Phase 1:

- No full fabric automation.
- Render provider artifacts and inventory.
- Expose management services.
- Track organization, site ID, system IP, role, and controller references.

Phase 2:

- Add cEdge bootstrap for system settings, VPN 0 transport, service VPN interfaces, SSH, and optional vBond reference.
- Keep certificate and serial authorization manual or explicitly documented.
- Add validation commands for `show sdwan running-config`, `show sdwan omp routes`, and `show sdwan control connections`.

Phase 3:

- Add vEdge bootstrap if image behavior is stable.
- Add controller bootstrap helpers only after x86 KNE validation.
- Consider API-assisted template attachment only after a repeatable lab workflow exists.

## Versa Target

### Devices

`versa-director`

- Purpose: Versa Director controller/API endpoint.
- Role support: `controller`.
- KNE model: `director`.
- Placeholder image: `vrnetlab/vr-versa-director:<version>`.
- Collector mapping: `versa_director_api`.
- Bootstrap path: metadata and service exposure first.

`versa-flexvnf`

- Purpose: Versa FlexVNF branch/edge.
- Role support: `edge`.
- KNE model: `flexvnf`.
- Placeholder image: `vrnetlab/vr-versa-flexvnf:<version>`.
- Collector mapping: `versa_flexvnf_ssh`.
- Bootstrap path: metadata and service exposure first.

### Versa Validation Rules

- A Versa inventory setup requires a Director and at least one FlexVNF edge.
- Do not add Analytics or extra controller/helper VMs until netlab needs to boot them or a collection workflow requires them.
- Versa image startup behavior must be validated before adding configuration templates.
- Versa Director API credentials should be represented as lab credentials or inventory hints, not hard-coded in device defaults.

### Versa Bootstrap Phases

Phase 1:

- Render Director/FlexVNF provider artifacts.
- Expose Director HTTPS and FlexVNF SSH.
- Emit role/site/system metadata.

Phase 2:

- Document and validate day-0 seed options for Director and FlexVNF.
- Add minimal templates only for stable startup inputs.

Phase 3:

- Add service-chain or policy metadata only when it maps to a repeatable lab behavior.

## Underlay Devices

Underlay support is required because SD-WAN labs need transport networks even when the SD-WAN appliances are metadata-only.

Initial underlay devices:

- `iol`
- `eos`
- `cat8000v` when explicitly used as underlay

Underlay requirements:

- Support ordinary netlab addressing, routing, and management.
- Allow `sdwan.role: underlay`.
- Reject edge/controller/manager/validator roles unless the device explicitly supports them.
- Avoid vendor-specific SD-WAN config generation.

## Provider Work

### KNE

KNE is the primary provider target for full SD-WAN labs because Cisco and Versa appliances are VM-shaped. Netlab should render KNE artifacts that contain enough metadata for an external KNE/KubeVirt runtime to choose the correct image and services.

KNE node mapping should include:

- `vendor`
- `model`
- `os`
- `image`
- management service exposure
- startup config path only when supported
- future VM/KubeVirt runtime hints

KNE service map:

- Cisco edge/controller/validator: SSH initially.
- Cisco manager: SSH and HTTPS initially.
- `cat8000v`: SSH, NETCONF, gNMI, gRIBI, P4RT, SNMP where the existing device contract supports them.
- Versa Director: HTTPS/API initially.
- Versa FlexVNF: SSH initially.

KNE should not assume proprietary image names beyond placeholder defaults. Users can override images through normal netlab defaults.

### Containerlab

Containerlab can be useful when a vrnetlab image exists and the host is x86 Linux. It should not be the assumed path for full Cisco SD-WAN on Apple Silicon.

Work needed:

- Decide whether SD-WAN appliance device files should also carry `clab` image/kind metadata.
- Avoid startup config promises unless the vrnetlab image supports them.
- Document that VM-in-container images are not container-native appliances.

### Libvirt

Libvirt remains useful for direct VM boot on x86 Linux.

Work needed:

- Add libvirt image names only when tested.
- Add create templates only if the appliance boot requirements are known.
- Keep libvirt out of the first pass unless it reduces friction compared to KNE.

## Image Registry Placeholders

Use generic placeholder names that match vrnetlab style and are easy to override:

```yaml
cat8000v:       vrnetlab/cisco_c8000v:<version>
cisco-edge:    vrnetlab/vr-vedge:<version>
cisco-smart:   vrnetlab/vr-vsmart:<version>
cisco_sdwan-validator:    vrnetlab/vr-vbond:<version>
cisco-manage:  vrnetlab/vr-vmanage:<version>
versa-director: vrnetlab/vr-versa-director:<version>
versa-flexvnf:  vrnetlab/vr-versa-flexvnf:<version>
```

The placeholders are not a claim that public images exist. They are registry contracts and test fixtures.

## Configuration Generation

Configuration generation should be staged.

Stage 1: metadata-only

- Normalize SD-WAN attributes.
- Render provider artifacts.
- Generate neutral inventory.
- Generate optional collector mapping.
- Do not generate unsupported vendor configs.

Stage 2: edge bootstrap

- Add cEdge bootstrap first because `cat8000v` already has netlab support and IOS-XE command surfaces are inspectable.
- Add vEdge and Versa FlexVNF bootstrap only after image behavior is validated.
- Include only management, transport, service VPN, system identity, and controller reference inputs.

Stage 3: controller/bootstrap helpers

- Add Cisco manager/controller/validator helpers only after the certificate/organization workflow is repeatable.
- Add Versa Director helpers only after day-0 seed behavior is validated.

Stage 4: policy intent

- Add OMP/data-policy/app-aware policy metadata for Cisco.
- Add Versa policy metadata only where it can be rendered or inventoried consistently.
- Keep policy modeling separate from initial fabric reachability.

## Files To Add Or Change

Module:

- `netsim/modules/sdwan.yml`
- `netsim/modules/sdwan.py`

Device definitions:

- `netsim/devices/cisco-edge.yml`
- `netsim/devices/cisco-smart.yml`
- `netsim/devices/cisco_sdwan-validator.yml`
- `netsim/devices/cisco-manage.yml`
- `netsim/devices/versa-director.yml`
- `netsim/devices/versa-flexvnf.yml`
- Existing underlay-capable device files for `features.sdwan.roles`

Provider:

- `netsim/providers/kne.py`
- Optional future containerlab/libvirt provider support if validated

Templates:

- Future `sdwan` templates only for validated bootstrap paths
- No templates for metadata-only appliances

Docs:

- This framework plan
- User-facing module docs once the module contract stabilizes
- Developer docs for SD-WAN template data once templates exist

Tests:

- Unit tests for module transform and validation
- KNE provider render tests for new devices and services
- Integration fixtures for Cisco and Versa inventory
- Error tests for invalid role/vendor/provider combinations

## Test Plan

Static tests:

- `python3 -m pytest -vvv tests/test_sdwan.py`
- `python3 -m pytest -vvv tests/test_kne_provider.py`
- `python3 -m pytest -vvv -k sdwan`
- `python3 -m mypy -p netsim`
- `ruff check netsim tests`
- `yamllint netsim/modules/sdwan.yml netsim/devices/cisco-*.yml netsim/devices/versa-*.yml tests/integration/sdwan/*.yml`

Transform tests:

- Cisco partial inventory: controller plus one or more edges.
- Cisco full fabric: manager, controller, validator, edge.
- Versa inventory: Director plus FlexVNF.
- Underlay-only devices with `sdwan.role: underlay`.
- Rejection of IOL/EOS as SD-WAN edge/controller.
- Rejection of Cisco device in Versa fabric and Versa device in Cisco fabric.

Provider tests:

- Render KNE textproto for each new device.
- Verify placeholder image propagation.
- Verify vendor/model/OS mapping.
- Verify SSH/HTTPS service exposure.
- Verify startup-config sidecar is absent for metadata-only appliances.
- Verify startup-config sidecar remains present for devices that support it.

Live tests:

- On macOS Apple Silicon: run transform/provider rendering only.
- On x86 Linux KNE: boot image placeholders after private images are built and pushed.
- For Cisco cEdge: verify SD-WAN commands are present and management collection works.
- For full Cisco fabric: verify controller connections and OMP state before claiming automation support.
- For Versa: verify Director API and FlexVNF SSH behavior before adding bootstrap templates.

## Implementation Roadmap

### Phase 0: Framework Contract

- Write this plan.
- Define neutral SD-WAN inventory shape.
- Decide whether the neutral inventory needs additional translation hooks in downstream orchestration.

### Phase 1: Module And Device Metadata

- Add `sdwan.yml` attributes and supported role feature names.
- Add `sdwan.py` transform/validation.
- Add Cisco and Versa device definitions.
- Mark known underlay devices with `features.sdwan.roles: [ underlay ]`.

### Phase 2: KNE Registry Plumbing

- Add KNE node maps for Cisco and Versa SD-WAN devices.
- Add KNE service maps.
- Add placeholder images using vrnetlab-style names.
- Add tests that verify image/model/OS/service contracts.

### Phase 3: Neutral Inventory Output

- Build `topology.sdwan.inventory`.
- Include node role, device, vendor, address, site ID, system IP, and optional connection hints.
- Derive consumer-specific setup data from the neutral inventory only where needed.
- Add snapshot/provider metadata tests.

### Phase 4: Cisco Edge Bootstrap

- Add conservative cEdge bootstrap support.
- Validate with `cat8000v` images on x86.
- Add fixture topologies with transport and service VPNs.

### Phase 5: Versa Minimal Bootstrap

- Validate Director/FlexVNF day-0 input behavior.
- Add minimal templates only when the startup path is repeatable.

### Phase 6: Policy And Advanced Fabric

- Add vendor-specific policy metadata under the `sdwan` namespace.
- Keep policy rendering optional and target-specific.
- Add more vendors only after their image, role, and bootstrap contracts are clear.

## Open Questions

- What exact KNE/KubeVirt hints are needed to distinguish VM appliances from container-native nodes?
- Do the Cisco controller images support reliable startup config injection, or do they require API/manual bootstrap?
- Which Versa images and versions have a stable day-0 seed path?
- How should credentials be represented without putting secrets into device defaults?
- Should service VPN intent reuse existing netlab VRF/VLAN structures, or remain SD-WAN-specific until policy rendering exists?

## Acceptance Criteria

The first useful version is complete when:

- A user can define Cisco SD-WAN or Versa nodes with SD-WAN roles.
- Netlab validates unsupported role/vendor combinations.
- KNE provider artifacts render with expected image, vendor, model, OS, and service metadata.
- A neutral SD-WAN inventory exists in `topology.sdwan.inventory` and is mirrored into the KNE snapshot metadata.
- Consumer-specific setup data can be derived without hard-coding project-specific assumptions.
- No generated configuration promises a vendor bootstrap path that has not been validated.
