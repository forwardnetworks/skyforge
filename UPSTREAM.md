# Upstream pins

Each series applies to the upstream version below and stays under that upstream's licence.

| Series | Upstream | Pinned to | Patches | Licence |
|---|---|---|---|---|
| netlab | https://github.com/ipspace/netlab | `ff52463be991e5e0c17ea396254625c25fef9aa6` (dev, 2026-11-02) | 59 | MIT-style |
| kne | https://github.com/openconfig/kne | `3fe74f99999ff8dbb685f45a6c18757ee0d2cf99` (master) | 50 | Apache-2.0 |
| koko | https://github.com/redhat-nfvpe/koko | `a18aa44814eabb542226e11c5568e8433810e4ab` (master) | 15 | Apache-2.0 |
| meshnet-cni | https://github.com/networkop/meshnet-cni | `d7c306c635cf73a44833e3ebdefb8b163a9aa784` (master) | 25 | BSD-3-Clause |
| arista-ceoslab-operator | https://github.com/aristanetworks/arista-ceoslab-operator | `v2.1.2` = `72c8685ee56c26087211846a5262d2954f543b59` | 20 | Apache-2.0 |
| catalystwan (Ansible collection) | https://github.com/cisco-en-programmability/ansible-collection-catalystwan | `b8adf36e2279624c6e3dfe573b65de62e2e25b96` (galaxy.yml 0.3.4; not released on Galaxy) | 5 | GPL-3.0 |
| catalystwan (Python SDK) | https://github.com/cisco-open/cisco-catalyst-wan-sdk | `catalystwan==0.41.5.dev2` | 1 | Apache-2.0 |
| nokia.grpc (Ansible collection) | https://github.com/nokia-steward/ansible-networking-collections | collection `1.0.2` | 1 | BSD-3-Clause |

netlab: the series makes KNE, our management pool and the `services` module the defaults, so netlab's golden-file tests (`tests/test_transformation.py`) fail by design. Run the rest from `tests/` (`PYTHONPATH=.. pytest`) with `ansible-core` and `passlib` installed. Two known failures: `test_a10_deploy_and_vip_advertise.py::test_every_a10_module_template_is_delivered_over_netmiko` and `test_plugin_sort_extra_config.py::test_sort_extra_config_is_first_seen_order`.
