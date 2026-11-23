(topology-otg)=
# Declared Traffic Flows (`otg:`)

`otg:` is a **topology-level block that is not part of upstream *netlab***. Its schema lives in the patched netlab defaults (`attributes.global.otg` in `netsim/defaults/attributes.yml`). Nothing goes in `module:` or `plugin:`, and no device configuration is rendered from it.

It declares the traffic flows a Keysight ixia-c generator (OTG, the Open Traffic Generator API) should run between its ports in this lab. Flows are declared in the topology because a flow is a statement about specific nodes and links and should version with them.

**Why it is not rendered as config.** The ixia-c controller takes no generated configuration at all -- its device definition says so and sets `netlab_initial: never`, because it is driven over the OTG API after boot. *netlab*'s job here is to CARRY and VALIDATE the declaration; whatever drives the controller reads it out of the compiled topology.

**Raw data, no verdict.** A flow says what to send and where; what came back is counters, reported as counters. Nothing here expresses a pass/fail -- that judgement is a separate concern and belongs to whatever consumes the measurement, not to the thing that takes it.

**Why it exists at all.** Monitoring collects over the management interface, so a completely dead data plane certifies green. A flow that is transmitted on one port and counted on another is a statement about the FABRIC between them -- the one claim no management-plane check can make.

```eval_rst
.. contents:: Table of Contents
   :depth: 2
   :local:
   :backlinks: none
```

## The nodes a flow needs

An ixia-c generator is two *netlab* device types, and a KNE node is a single-container pod, so the API and each port are separate NODES:

* `keysight_ixia-c-controller` -- the API surface. Exactly one per lab. **No data links**: flows are driven through this node, and the engines are reached through it rather than directly.
* `keysight_ixia-c-traffic-engine` -- one per generator PORT, owning its one data interface. Wire each one to the device under test with an ordinary link.

The combined `ixia-c-one` image cannot be used here: it is docker-in-docker, reaches Running, fails with "could not wait for docker daemon" and listens on nothing. The component images are ordinary containers.

The node names (`otgctl`, `otg1`) are the author's choice.

## Attributes

Every key below is the schema in `attributes.global.otg`. *netlab* refuses a key that is not listed here (`Incorrect global attribute ...`).

| Attribute | Type | Meaning |
| --- | --- | --- |
| `controller` | string | Which node serves the OTG API. Optional when the topology has exactly one `keysight_ixia-c-controller`, which is the normal case.  |
| `flows` | list of dict | The flows. |

### `otg.flows[]`

| Attribute | Type | Meaning |
| --- | --- | --- |
| `name` | string | Names the flow in every metric row that comes back, so it has to survive round-tripping through the OTG API unchanged. Must be unique: metric rows are keyed by name, so two flows sharing one are indistinguishable in the result. |
| `tx` | string | The traffic-engine NODE that transmits. A **node name** (`otg1`), not an address or an interface. Ports rather than addresses, because a port is what the generator owns. |
| `rx` | string | The traffic-engine NODE that receives. Same rules as `tx`. |
| `src` | ipv4 address | Override the source address derived from the `tx` port. Needed when the flow should carry an address the generator does not own -- testing a route rather than the adjacency it sits on. A bare address (`10.1.0.1`), no mask. |
| `dst` | ipv4 address | Override the destination address derived from the `rx` port. Same rules as `src`. |
| `rate_pps` | int >= 1 | Offered load, packets per second per port. Unset leaves the controller's default. |
| `size` | int 64..9000 | Fixed frame size in bytes. Unset leaves the controller's default. |
| `packets` | int >= 0 | `0` (the default) means continuous: start it and it runs until stopped. A fixed count is what you want for a measurement that has to end on its own. |
| `protocol` | `udp`, `tcp` or `icmp` | Unset means Ethernet/IPv4 only, which is enough to prove forwarding. `icmp` sends echo requests. |
| `src_port` | int 1..65535 | Transport source port. Only applied with `protocol: udp` or `tcp`; silently unused otherwise. |
| `dst_port` | int 1..65535 | Transport destination port. Same rule as `src_port`. |

There is no MAC field in the topology: both MACs of a port-to-port flow are *netlab*'s render of the two ports.

## Example

The smallest topology that can prove a data plane carries traffic. Three generator ports hang off one layer-2 switch with two VLANs: `otg1` and `otg2` share one, `otg3` is alone in the other. The first two flows have to be switched by `sw` to arrive, so `rx` on the far side is evidence about the fabric; the third is the **negative control** and must not arrive.

```yaml
provider: kne
defaults.device: eos
module: [ vlan ]

vlans:
  engines:
    mode: bridge
  island:
    mode: bridge
    prefix:                  # same prefix as `engines` on purpose
      ipv4: 172.16.0.0/24
      allocation: id_based

nodes:
  otgctl:
    device: keysight_ixia-c-controller   # the API; no data links
    module: []
  otg1:
    device: keysight_ixia-c-traffic-engine
    module: []
  otg2:
    device: keysight_ixia-c-traffic-engine
    module: []
  otg3:
    device: keysight_ixia-c-traffic-engine
    module: []
  sw:

links:
- sw:
  otg1:
  vlan.access: engines
- sw:
  otg2:
  vlan.access: engines
- sw:
  otg3:
  vlan.access: island

otg:
  controller: otgctl
  flows:
    - {name: otg1-to-otg2, tx: otg1, rx: otg2, rate_pps: 100, size: 512, packets: 500, protocol: udp}
    - {name: otg2-to-otg1, tx: otg2, rx: otg1, rate_pps: 100, size: 512, packets: 500, protocol: udp}
    - {name: otg1-to-otg3-isolated, tx: otg1, rx: otg3, rate_pps: 100, size: 512, packets: 500, protocol: udp}
```

Read the three flows together: `tx` is 500 on all of them, `rx` is 500, 500 and 0. Zero everywhere means a dead fabric or a dead generator; 500 on the isolated one means the VLAN leaks. The isolated flow shares a subnet with the others on purpose, so a zero on it is the switch's VLAN doing its job and not a routing artifact. Fixed packet counts (`packets: 500`) make `tx` exactly known and the run end on its own.

## Validation

`netlab create` validates the block against `attributes.global.otg`: unknown keys, out-of-range numbers, a `protocol` outside `udp`/`tcp`/`icmp`, or a `src`/`dst` with a mask all fail the compile.

## Pitfalls

* **`tx` and `rx` are traffic-engine node names.** Not IP addresses, not interface names, not the router's name. The schema types them as `str`, so *netlab* cannot catch a typo at compile time.
* **Face the two ports at each other and the flow proves nothing.** The same flow passes while proving only that the generator can talk to itself. Put the device under test between `tx` and `rx`.
* **The controller gets no data links.** The controller node is the API and the traffic engines own the ports. A link on the controller is not a generator port and cannot be named in `tx`/`rx`.
* **`otg:` renders no device config.** If the generator holds nothing after bring-up, the question is not "did netlab render the flows" -- it never does.
