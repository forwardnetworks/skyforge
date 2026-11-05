import os

from netsim import __version__
from netsim.utils import log, files as _files,strings
from netsim import data

from box import Box

def init(topology: Box) -> None:
  if __version__ >= '1.9.2':
    return

  if topology.defaults.device not in ['eos','frr']:
    log.error(
      f'EBGP ECMP configuration is not available for device {topology.defaults.device}',
      category=Warning,
      module='custom')
    topology.groups.get('leafs',{}).pop('config',None)

def cleanup(topology: Box) -> None:
  # SAME VERSION GUARD AS init() ABOVE, AND FOR THE SAME REASON. This plugin
  # back-fills a check netlab gained natively in 1.9.2; on 26.08 netlab already
  # reports a missing custom config itself ("Cannot find <name> configuration
  # template for <node>/device <device>", module `configs`).
  #
  # Left unguarded it does not merely duplicate that check, it CONTRADICTS it.
  # The candidate list below joins the config name to the file with a literal
  # dot -- `ebgp.multihop/eos.j2` -- but netlab resolves a dotted config name
  # through `custom_slash_config`, which turns the dots into directory
  # separators. The template this topology needs ships with netlab as
  # netsim/extra/ebgp/multihop/eos.j2, so netlab finds it and this plugin does
  # not, and the run died on a config that was present all along:
  #   "Custom configuration ebgp.multihop is missing for device eos
  #    (nodes L1,L2,L3,L4,S1,S2)"
  # It then STRIPPED ebgp.multihop from every node, so even a downgraded
  # warning would have silently removed the multihop sessions the lab is about.
  if __version__ >= '1.9.2':
    return

  missing = data.get_empty_box()
  for ndata in topology.nodes.values():
    if not 'config' in ndata:
      continue

    candidate_files = [ 
      f'{ndata.name}.{ndata.device}.j2',
      f'{ndata.name}.j2',
      f'{ndata.device}.j2' ]

    for cfg in list(ndata.config):
      for cfg_candidate in candidate_files:
        cfg_file = _files.find_file(f'{cfg}/{cfg_candidate}',topology.defaults.paths.custom.dirs)
      
        if cfg_file:
          break

      if cfg_file:
        break
      
      cfg_id = strings.make_id(cfg)
      missing[cfg_id].item = cfg
      data.append_to_list(missing[cfg_id].devices,ndata.device,ndata.name)
      ndata.config = [ f for f in ndata.config if f != cfg ]

  hint = "You will have to configure the missing functionality yourself"
  for m_config in missing.values():
    for m_device,m_nodes in m_config.devices.items():
      log.error(
        f'Custom configuration {m_config.item} is missing for device {m_device} (nodes {",".join(m_nodes)})',
        category=Warning,
        more_hints=hint,
        module='check.config')
      hint = None
