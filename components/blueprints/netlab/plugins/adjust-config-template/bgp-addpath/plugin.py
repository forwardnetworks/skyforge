'''
Custom plugin for BGP anycast scenario

Removes IBGP sessions from the members of the 'anycast' group
'''

import sys
from box import Box
from netsim.augment import topology
from netsim import api

def init(topo: Box) -> None:
  # netlab 26.08 tightening: module attributes are a dict of type definitions,
  # not a list (and netsim.common is gone) — declare the custom attribute.
  topo.defaults.bgp.attributes.node.add_path = 'bool'

def post_transform(topo: Box) -> None:
  config_name = api.get_config_name(globals())
  for name,node in topo.nodes.items():
    if 'add_path' in node.get('bgp',{}):
      api.node_config(node,config_name)
