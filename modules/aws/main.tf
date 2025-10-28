locals {
  vpc_map     = var.region_config.vpcs
  gwlb_config = try(var.region_config.gwlb_paloalto, null)

  subnet_blueprints = flatten([
    for vpc_name, vpc in local.vpc_map : [
      for tier_index, tier in vpc.tiers : [
        for az_index, az in var.region_config.availability_zones : {
          key        = "${vpc_name}-${tier}-${replace(az, "-", "")}"
          vpc_name   = vpc_name
          tier       = tier
          tier_index = tier_index
          az_index   = az_index
          az         = az
          cidr_block = cidrsubnet(vpc.cidr_block, vpc.tier_subnet_prefix, tier_index + az_index * length(vpc.tiers))
        }
      ]
    ]
  ])

  subnet_map = { for b in local.subnet_blueprints : b.key => b }

  inspection_vpc_candidates = {
    for name, vpc in local.vpc_map :
    name => vpc if contains(vpc.tiers, "inspection")
  }

  inspection_vpc_name = try(keys(local.inspection_vpc_candidates)[0], null)

  inspection_subnet_ids = local.inspection_vpc_name == null ? [] : [
    for key, subnet in aws_subnet.this :
    subnet.id if local.subnet_map[key].vpc_name == local.inspection_vpc_name && local.subnet_map[key].tier == "inspection"
  ]

  third_party_firewalls_config = try(var.region_config.third_party_firewalls, null)
  fortinet_config              = try(local.third_party_firewalls_config.fortinet, null)
  checkpoint_config            = try(local.third_party_firewalls_config.checkpoint, null)

  fortinet_enabled   = local.inspection_vpc_name != null && local.fortinet_config != null && try(local.fortinet_config.enable, false) && try(local.fortinet_config.ami_id, null) != null
  checkpoint_enabled = local.inspection_vpc_name != null && local.checkpoint_config != null && try(local.checkpoint_config.enable, false) && try(local.checkpoint_config.ami_id, null) != null
  name_prefix        = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
}

data "aws_ami" "paloalto_gwlb" {
  count       = local.gwlb_config != null && try(local.gwlb_config.ami_id, null) == null && try(local.gwlb_config.ami_product_code, null) != null ? 1 : 0
  most_recent = true
  owners      = ["aws-marketplace"]

  filter {
    name   = "product-code"
    values = [local.gwlb_config.ami_product_code]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

locals {
  gwlb_ami_candidates = local.gwlb_config == null ? [] : [
    try(local.gwlb_config.ami_id, null),
    try(data.aws_ami.paloalto_gwlb[0].image_id, null)
  ]
  gwlb_ami_non_null    = [for ami in local.gwlb_ami_candidates : ami if ami != null && ami != ""]
  gwlb_ami_id_resolved = length(local.gwlb_ami_non_null) > 0 ? local.gwlb_ami_non_null[0] : null
}

resource "aws_vpc" "this" {
  for_each = local.vpc_map

  cidr_block                       = each.value.cidr_block
  assign_generated_ipv6_cidr_block = true
  enable_dns_support               = true
  enable_dns_hostnames             = true

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-vpc"
    SkyforgeVPC    = each.key
    SkyforgeRegion = var.region_key
  })
}

resource "aws_subnet" "this" {
  for_each = local.subnet_map

  vpc_id                  = aws_vpc.this[each.value.vpc_name].id
  availability_zone       = each.value.az
  cidr_block              = each.value.cidr_block
  map_public_ip_on_launch = contains(["ingress", "public", "frontend"], each.value.tier)

  tags = merge(var.default_tags, {
    Name             = "${local.name_prefix}-${var.region_key}-${each.value.vpc_name}-${each.value.tier}-${each.value.az}"
    Tier             = each.value.tier
    AvailabilityZone = each.value.az
    SkyforgeRegion   = var.region_key
    SkyforgeVPC      = each.value.vpc_name
  })
}

resource "aws_internet_gateway" "this" {
  for_each = { for vpc_name, vpc in local.vpc_map : vpc_name => vpc if contains([for tier in vpc.tiers : tier], "ingress") }

  vpc_id = aws_vpc.this[each.key].id

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-${each.key}-igw"
  })
}

resource "aws_ec2_transit_gateway" "this" {
  count = var.region_config.enable_transit_gateway ? 1 : 0

  description                     = "Skyforge ${var.region_key} transit gateway"
  amazon_side_asn                 = 64512
  auto_accept_shared_attachments  = "enable"
  default_route_table_association = "enable"
  default_route_table_propagation = "enable"
  vpn_ecmp_support                = "enable"

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-tgw"
  })
}

resource "aws_ec2_transit_gateway_vpc_attachment" "this" {
  for_each = var.region_config.enable_transit_gateway ? local.vpc_map : {}

  subnet_ids = [
    for key, subnet in aws_subnet.this :
    subnet.id if local.subnet_map[key].vpc_name == each.key && local.subnet_map[key].tier == each.value.tiers[0]
  ]

  transit_gateway_id = aws_ec2_transit_gateway.this[0].id
  vpc_id             = aws_vpc.this[each.key].id

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-${each.key}-tgw-attachment"
  })

  depends_on = [aws_subnet.this]
}

resource "aws_ec2_transit_gateway_connect" "this" {
  for_each = local.transit_gateway_connect_active ? { primary = true } : {}

  transit_gateway_id      = aws_ec2_transit_gateway.this[0].id
  transport_attachment_id = aws_ec2_transit_gateway_vpc_attachment.this[local.transit_gateway_transport_vpc].id
  protocol                = lower(try(local.transit_gateway_connect_config.protocol, "gre"))

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-tgw-connect"
  })
}

resource "aws_ec2_transit_gateway_connect_peer" "this" {
  for_each = local.transit_gateway_connect_active ? { primary = true } : {}

  transit_gateway_attachment_id = aws_ec2_transit_gateway_connect.this["primary"].id
  peer_address                  = local.transit_gateway_connect_connector_enabled && length(module.transit_gateway_connect_connector) > 0 ? module.transit_gateway_connect_connector[0].private_ip : local.transit_gateway_connect_peer_address_input
  inside_cidr_blocks            = [local.transit_gateway_connect_inside_cidr]
  bgp_asn                       = coalesce(local.transit_gateway_connect_bgp_asn, 65001)
  transit_gateway_address       = local.transit_gateway_connect_tgw_ip

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-tgw-connect-peer"
  })
}

module "paloalto_gwlb" {
  count = var.region_config.enable_gateway_lb && local.inspection_vpc_name != null && local.gwlb_ami_id_resolved != null ? 1 : 0

  source = "./gwlb_paloalto"

  providers = {
    aws = aws
  }

  vpc_id               = aws_vpc.this[local.inspection_vpc_name].id
  subnet_ids           = local.inspection_subnet_ids
  tags                 = var.default_tags
  firewall_ami_id      = local.gwlb_ami_id_resolved
  instance_type        = try(local.gwlb_config.instance_type, null)
  instance_count       = try(local.gwlb_config.instance_count, null)
  iam_instance_profile = try(local.gwlb_config.iam_instance_profile, null)
  user_data            = try(local.gwlb_config.user_data, null)
  bootstrap            = try(local.gwlb_config.bootstrap, null)
  name_prefix          = try(local.gwlb_config.name_prefix, "${local.name_prefix}-palo")
  allowed_principals   = try(local.gwlb_config.allowed_principals, [])
}

module "transit_gateway_connect_connector" {
  count = local.transit_gateway_connect_connector_enabled ? 1 : 0

  source = "./fortinet_connect"

  providers = {
    aws = aws
  }

  vpc_id    = aws_vpc.this[local.transit_gateway_transport_vpc].id
  subnet_id = local.transit_gateway_connect_connector_subnet_id
  tags      = var.default_tags
  tags_extra = {
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "tgw-connect"
  }
  name_prefix          = try(local.transit_gateway_connect_connector_config.name_prefix, "${local.name_prefix}-${var.region_key}-fortinet")
  ami_id               = try(local.transit_gateway_connect_connector_config.ami_id, null)
  ami_product_code     = try(local.transit_gateway_connect_connector_config.ami_product_code, "cyxc6cynd1msb41uz9fou0byi")
  instance_type        = try(local.transit_gateway_connect_connector_config.instance_type, "c5.xlarge")
  iam_instance_profile = try(local.transit_gateway_connect_connector_config.iam_instance_profile, null)
  key_name             = try(local.transit_gateway_connect_connector_config.key_name, null)
  inside_cidr          = local.transit_gateway_connect_inside_cidr
  peer_bgp_asn         = coalesce(local.transit_gateway_connect_bgp_asn, 65001)
  transit_gateway_asn  = try(aws_ec2_transit_gateway.this[0].amazon_side_asn, 64512)
  advertised_prefixes  = local.transit_gateway_connect_advertised_prefixes
  management_cidrs     = try(local.transit_gateway_connect_connector_config.management_cidrs, ["10.0.0.0/8", "192.168.0.0/16"])
  user_data            = try(local.transit_gateway_connect_connector_config.user_data, null)
  admin_username       = try(local.transit_gateway_connect_connector_config.admin_username, "fortinetadmin")
  admin_password       = try(local.transit_gateway_connect_connector_config.admin_password, null)
}

locals {
  tier_subnets_by_vpc = {
    for vpc_name, vpc in local.vpc_map :
    vpc_name => {
      for tier in vpc.tiers :
      tier => [
        for key, subnet in aws_subnet.this :
        subnet.id if local.subnet_map[key].vpc_name == vpc_name && local.subnet_map[key].tier == tier
      ]
    }
  }

  vpc_ids = { for name, vpc in aws_vpc.this : name => vpc.id }

  app_stack_config                             = try(var.region_config.app_stack, null)
  app_stack_enabled                            = local.app_stack_config != null && try(local.app_stack_config.enable, false)
  app_frontend_subnets                         = local.app_stack_enabled ? lookup(lookup(local.tier_subnets_by_vpc, local.app_stack_config.frontend_vpc, {}), local.app_stack_config.frontend_tier, []) : []
  app_app_subnets                              = local.app_stack_enabled ? lookup(lookup(local.tier_subnets_by_vpc, local.app_stack_config.app_vpc, {}), local.app_stack_config.app_tier, []) : []
  app_data_subnets                             = local.app_stack_enabled ? lookup(lookup(local.tier_subnets_by_vpc, local.app_stack_config.data_vpc, {}), local.app_stack_config.data_tier, []) : []
  app_frontend_vpc_id                          = local.app_stack_enabled ? lookup(local.vpc_ids, local.app_stack_config.frontend_vpc, null) : null
  app_app_vpc_id                               = local.app_stack_enabled ? lookup(local.vpc_ids, local.app_stack_config.app_vpc, null) : null
  app_data_vpc_id                              = local.app_stack_enabled ? lookup(local.vpc_ids, local.app_stack_config.data_vpc, null) : null
  app_stack_ready                              = local.app_stack_enabled && length(local.app_frontend_subnets) > 0 && length(local.app_app_subnets) > 0 && length(local.app_data_subnets) > 0 && local.app_frontend_vpc_id != null && local.app_app_vpc_id != null && local.app_data_vpc_id != null
  transit_gateway_connect_config               = try(var.region_config.transit_gateway_connect, null)
  transit_gateway_connect_enabled              = var.region_config.enable_transit_gateway && local.transit_gateway_connect_config != null && try(local.transit_gateway_connect_config.enable, false)
  transit_gateway_transport_vpc                = local.transit_gateway_connect_enabled ? local.transit_gateway_connect_config.transport_vpc : null
  transit_gateway_transport_attachment         = local.transit_gateway_connect_enabled ? lookup(aws_ec2_transit_gateway_vpc_attachment.this, local.transit_gateway_transport_vpc, null) : null
  transit_gateway_transport_attachment_exists  = local.transit_gateway_transport_attachment != null
  transit_gateway_connect_subnet_ids           = local.transit_gateway_transport_attachment_exists ? lookup(lookup(local.tier_subnets_by_vpc, local.transit_gateway_connect_config.transport_vpc, {}), local.transit_gateway_connect_config.subnet_tier, []) : []
  transit_gateway_connect_inside_cidr          = local.transit_gateway_connect_enabled ? try(local.transit_gateway_connect_config.inside_cidr, null) : null
  transit_gateway_connect_peer_address_input   = try(local.transit_gateway_connect_config.peer_address, null)
  transit_gateway_connect_bgp_asn              = local.transit_gateway_connect_enabled ? try(local.transit_gateway_connect_config.peer_bgp_asn, 65001) : null
  transit_gateway_connect_connector_config     = local.transit_gateway_connect_enabled ? try(local.transit_gateway_connect_config.connector, null) : null
  transit_gateway_connect_connector_enabled    = local.transit_gateway_connect_enabled && local.transit_gateway_connect_connector_config != null && try(local.transit_gateway_connect_connector_config.enable, true)
  transit_gateway_connect_connector_subnet_id  = local.transit_gateway_connect_connector_enabled && length(local.transit_gateway_connect_subnet_ids) > 0 ? local.transit_gateway_connect_subnet_ids[0] : null
  transit_gateway_connect_advertised_overrides = try(local.transit_gateway_connect_connector_config.advertised_prefixes, [])
  transit_gateway_connect_default_advertisements = distinct([
    for name, vpc in local.vpc_map :
    vpc.cidr_block
  ])
  transit_gateway_connect_advertised_prefixes = length(local.transit_gateway_connect_advertised_overrides) > 0 ? local.transit_gateway_connect_advertised_overrides : local.transit_gateway_connect_default_advertisements
  transit_gateway_connect_active              = local.transit_gateway_connect_enabled && local.transit_gateway_connect_inside_cidr != null && (local.transit_gateway_connect_connector_enabled || local.transit_gateway_connect_peer_address_input != null)
  extra_security_groups                       = try(var.region_config.security_groups, [])
  extra_security_group_map                    = { for sg in local.extra_security_groups : sg.name => sg }
  extra_network_acls                          = try(var.region_config.network_acls, [])
  extra_network_acl_map                       = { for acl in local.extra_network_acls : acl.name => acl }
}

locals {
  interface_endpoint_definitions = try(var.region_config.vpc_endpoints.interface, [])
  interface_endpoint_map         = { for ep in local.interface_endpoint_definitions : ep.name => ep }
  interface_endpoint_subnets = {
    for ep in local.interface_endpoint_definitions :
    ep.name => distinct(flatten([
      for tier in ep.subnet_tiers :
      lookup(lookup(local.tier_subnets_by_vpc, ep.vpc, {}), tier, [])
    ]))
  }
  interface_endpoint_allowed_cidrs = {
    for ep in local.interface_endpoint_definitions :
    ep.name => length(try(ep.allowed_cidrs, [])) > 0 ? ep.allowed_cidrs : [var.region_config.cidr_block]
  }
  interface_endpoint_service_names = {
    for ep in local.interface_endpoint_definitions :
    ep.name => coalesce(try(ep.service_name, null), format("com.amazonaws.%s.%s", var.region_key, try(ep.service, ep.name)))
  }
  nat_gateway_definitions    = try(var.region_config.nat_gateways, [])
  nat_gateway_definition_map = { for nat in local.nat_gateway_definitions : nat.name => nat }
  nat_gateway_public_subnet_map = {
    for nat in local.nat_gateway_definitions :
    nat.name => lookup(lookup(local.tier_subnets_by_vpc, nat.vpc, {}), nat.public_subnet_tier, [])
  }
  nat_gateway_private_subnet_pairs = flatten([
    for nat in local.nat_gateway_definitions : [
      for tier in nat.private_subnet_tiers : {
        key        = "${nat.name}__${tier}"
        nat_name   = nat.name
        vpc_name   = nat.vpc
        tier       = tier
        subnet_ids = lookup(lookup(local.tier_subnets_by_vpc, nat.vpc, {}), tier, [])
      }
    ]
  ])
  nat_gateway_private_subnet_map = {
    for entry in local.nat_gateway_private_subnet_pairs :
    entry.key => entry
  }
  nat_gateway_private_associations = flatten([
    for key, cfg in local.nat_gateway_private_subnet_map : [
      for subnet_id in cfg.subnet_ids : {
        key       = "${key}__${subnet_id}"
        route_key = key
        subnet_id = subnet_id
        nat_name  = cfg.nat_name
      }
    ]
  ])
  nat_gateway_private_association_map = {
    for assoc in local.nat_gateway_private_associations :
    assoc.key => {
      route_key = assoc.route_key
      subnet_id = assoc.subnet_id
      nat_name  = assoc.nat_name
    }
  }
  vpc_peering_definitions = try(var.region_config.vpc_peerings, [])
  vpc_peering_map         = { for peer in local.vpc_peering_definitions : peer.name => peer }
  managed_prefix_lists    = try(var.region_config.prefix_lists, [])
  managed_prefix_list_map = { for pl in local.managed_prefix_lists : pl.name => pl }
}

locals {
  transit_gateway_connect_peer_address_effective = local.transit_gateway_connect_connector_enabled && length(module.transit_gateway_connect_connector) > 0 ? module.transit_gateway_connect_connector[0].private_ip : local.transit_gateway_connect_peer_address_input
  transit_gateway_connect_tgw_ip                 = local.transit_gateway_connect_active ? cidrhost(local.transit_gateway_connect_inside_cidr, 1) : null
  transit_gateway_connect_connector_metadata = local.transit_gateway_connect_connector_enabled && length(module.transit_gateway_connect_connector) > 0 ? {
    instance_id       = module.transit_gateway_connect_connector[0].instance_id
    private_ip        = module.transit_gateway_connect_connector[0].private_ip
    management_ip     = module.transit_gateway_connect_connector[0].management_ip
    inside_ip         = module.transit_gateway_connect_connector[0].inside_ip
    security_group_id = module.transit_gateway_connect_connector[0].security_group_id
    tgw_inside_ip     = module.transit_gateway_connect_connector[0].tgw_inside_ip
    admin_credentials = module.transit_gateway_connect_connector[0].admin_credentials
  } : null
  transit_gateway_attachment_id_map = local.transit_gateway_enabled ? { for name, attachment in aws_ec2_transit_gateway_vpc_attachment.this : name => attachment.id } : {}
  transit_gateway_attachment_ids_inspected = local.transit_gateway_connect_active ? {
    for name, attachment_id in local.transit_gateway_attachment_id_map :
    name => attachment_id if name != local.transit_gateway_transport_vpc
  } : {}
  transit_gateway_inspection_routes = local.transit_gateway_connect_active ? {
    for name, vpc in local.vpc_map :
    name => vpc.cidr_block if name != local.transit_gateway_transport_vpc
  } : {}
}

resource "aws_ec2_transit_gateway_route_table" "inspection" {
  count = local.transit_gateway_connect_active ? 1 : 0

  transit_gateway_id = aws_ec2_transit_gateway.this[0].id

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-tgw-inspection"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "tgw-route-table"
  })
}

resource "aws_ec2_transit_gateway_route_table" "appliance" {
  count = local.transit_gateway_connect_active ? 1 : 0

  transit_gateway_id = aws_ec2_transit_gateway.this[0].id

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-tgw-appliance"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "tgw-route-table"
  })
}

resource "aws_ec2_transit_gateway_route_table_association" "inspection_vpcs" {
  for_each = local.transit_gateway_connect_active ? local.transit_gateway_attachment_ids_inspected : {}

  transit_gateway_route_table_id = aws_ec2_transit_gateway_route_table.inspection[0].id
  transit_gateway_attachment_id  = each.value
}

resource "aws_ec2_transit_gateway_route_table_association" "appliance" {
  count = local.transit_gateway_connect_active ? 1 : 0

  transit_gateway_route_table_id = aws_ec2_transit_gateway_route_table.appliance[0].id
  transit_gateway_attachment_id  = aws_ec2_transit_gateway_connect.this["primary"].id
}

resource "aws_ec2_transit_gateway_route_table_propagation" "appliance_vpcs" {
  for_each = local.transit_gateway_connect_active ? local.transit_gateway_attachment_ids_inspected : {}

  transit_gateway_route_table_id = aws_ec2_transit_gateway_route_table.appliance[0].id
  transit_gateway_attachment_id  = each.value
}

resource "aws_ec2_transit_gateway_route" "inspection_to_connect" {
  for_each = local.transit_gateway_connect_active ? local.transit_gateway_inspection_routes : {}

  transit_gateway_route_table_id = aws_ec2_transit_gateway_route_table.inspection[0].id
  destination_cidr_block         = each.value
  transit_gateway_attachment_id  = aws_ec2_transit_gateway_connect.this["primary"].id
}

locals {
  network_firewall_inspection_vpc = local.inspection_vpc_name
  network_firewall_subnet_ids     = local.inspection_subnet_ids
  network_firewall_enabled        = var.region_config.enable_network_firewall && local.network_firewall_inspection_vpc != null && length(local.network_firewall_subnet_ids) > 0
  network_firewall_config         = try(var.region_config.network_firewall, null)
  network_firewall_stateful_rules = local.network_firewall_config != null && length(try(local.network_firewall_config.stateful_rules, [])) > 0 ? [
    for rule in local.network_firewall_config.stateful_rules : format("%s %s %s %s -> %s %s (sid:%s; rev:%s; msg:\"%s\";)",
      lower(rule.action),
      lower(rule.protocol),
      rule.source,
      try(rule.source_port, "any"),
      rule.destination,
      try(rule.destination_port, "any"),
      rule.sid,
      try(rule.rev, 1),
    replace(try(rule.description, rule.name), "\"", "'"))
    ] : [
    "pass tcp 10.0.0.0/8 any -> 10.10.0.0/16 80 (sid:100; rev:1; msg:\"Allow HTTP east-west\";)",
    "pass tcp 10.0.0.0/8 any -> 10.10.0.0/16 443 (sid:101; rev:1; msg:\"Allow HTTPS east-west\";)",
    "pass udp 10.0.0.0/8 any -> 10.10.56.0/21 514 (sid:150; rev:1; msg:\"Permit syslog to logging\";)",
    "alert tcp any any -> any 445 (sid:250; rev:1; msg:\"Watch SMB traffic\";)",
    "drop tcp any any -> any 23 (sid:300; rev:1; msg:\"Block Telnet\";)"
  ]
  network_firewall_stateful_rules_string = join("\n", local.network_firewall_stateful_rules)
  network_firewall_stateless_rules = local.network_firewall_config != null && length(try(local.network_firewall_config.stateless_rules, [])) > 0 ? [
    for rule in local.network_firewall_config.stateless_rules : {
      priority          = rule.priority
      action            = rule.action
      source_cidrs      = coalesce(try(rule.source_cidrs, null), ["0.0.0.0/0"])
      destination_cidrs = coalesce(try(rule.destination_cidrs, null), ["0.0.0.0/0"])
      source_ports      = coalesce(try(rule.source_ports, null), [])
      destination_ports = coalesce(try(rule.destination_ports, null), [])
      protocols         = [for proto in coalesce(try(rule.protocols, null), ["tcp"]) : tonumber(lookup(local.acl_protocol_lookup, lower(tostring(proto)), tostring(proto)))]
    }
    ] : [
    {
      priority          = 10
      action            = "aws:pass"
      source_cidrs      = ["0.0.0.0/0"]
      destination_cidrs = ["0.0.0.0/0"]
      source_ports      = []
      destination_ports = [{ from_port = 443, to_port = 443 }]
      protocols         = [6]
    },
    {
      priority          = 15
      action            = "aws:pass"
      source_cidrs      = ["10.0.0.0/8"]
      destination_cidrs = ["10.0.0.0/8"]
      source_ports      = []
      destination_ports = [{ from_port = 53, to_port = 53 }]
      protocols         = [17]
    },
    {
      priority          = 100
      action            = "aws:drop"
      source_cidrs      = ["0.0.0.0/0"]
      destination_cidrs = ["0.0.0.0/0"]
      source_ports      = []
      destination_ports = [{ from_port = 23, to_port = 23 }]
      protocols         = [6]
    }
  ]
  network_firewall_stateless_default_actions          = coalesce(try(local.network_firewall_config.default_actions.forward, null), ["aws:forward_to_sfe"])
  network_firewall_stateless_fragment_default_actions = coalesce(try(local.network_firewall_config.default_actions.fragment, null), ["aws:forward_to_sfe"])
  acl_protocol_lookup = {
    tcp  = "6"
    udp  = "17"
    icmp = "1"
    gre  = "47"
    all  = "-1"
    "-1" = "-1"
  }
}

locals {
  transit_gateway_enabled = var.region_config.enable_transit_gateway
  transit_gateway_connect_metadata = local.transit_gateway_connect_active ? {
    attachment_id             = aws_ec2_transit_gateway_connect.this["primary"].id
    peer_id                   = aws_ec2_transit_gateway_connect_peer.this["primary"].id
    peer_address              = local.transit_gateway_connect_peer_address_effective
    inside_cidr               = local.transit_gateway_connect_inside_cidr
    transit_gateway_ip        = local.transit_gateway_connect_tgw_ip
    peer_bgp_asn              = coalesce(local.transit_gateway_connect_bgp_asn, 65001)
    connector                 = local.transit_gateway_connect_connector_metadata
    inspection_route_table_id = aws_ec2_transit_gateway_route_table.inspection[0].id
    appliance_route_table_id  = aws_ec2_transit_gateway_route_table.appliance[0].id
  } : null
  aws_vnf_link_map = local.transit_gateway_enabled ? {
    for link in var.vnf_links :
    link.site => link
    if try(link.customer_gateway_ipv4, null) != null
  } : {}
}

resource "aws_networkfirewall_rule_group" "stateful" {
  count = local.network_firewall_enabled ? 1 : 0

  capacity = 100
  name     = "${local.name_prefix}-${var.region_key}-stateful"
  type     = "STATEFUL"

  rule_group {
    rules_source {
      rules_string = local.network_firewall_stateful_rules_string
    }
  }
}

resource "aws_networkfirewall_rule_group" "stateless" {
  count = local.network_firewall_enabled ? 1 : 0

  capacity = 100
  name     = "${local.name_prefix}-${var.region_key}-stateless"
  type     = "STATELESS"

  rule_group {
    rules_source {
      stateless_rules_and_custom_actions {
        dynamic "stateless_rule" {
          for_each = local.network_firewall_stateless_rules
          content {
            priority = stateless_rule.value.priority

            rule_definition {
              actions = [stateless_rule.value.action]

              match_attributes {
                protocols = stateless_rule.value.protocols

                dynamic "source" {
                  for_each = stateless_rule.value.source_cidrs
                  content {
                    address_definition = source.value
                  }
                }

                dynamic "destination" {
                  for_each = stateless_rule.value.destination_cidrs
                  content {
                    address_definition = destination.value
                  }
                }

                dynamic "source_port" {
                  for_each = stateless_rule.value.source_ports
                  content {
                    from_port = source_port.value.from_port
                    to_port   = source_port.value.to_port
                  }
                }

                dynamic "destination_port" {
                  for_each = stateless_rule.value.destination_ports
                  content {
                    from_port = destination_port.value.from_port
                    to_port   = destination_port.value.to_port
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}

resource "aws_networkfirewall_firewall_policy" "this" {
  count = local.network_firewall_enabled ? 1 : 0

  name = "${local.name_prefix}-${var.region_key}-firewall-policy"

  firewall_policy {
    stateful_engine_options {
      rule_order = "DEFAULT_ACTION_ORDER"
    }

    stateful_rule_group_reference {
      resource_arn = aws_networkfirewall_rule_group.stateful[0].arn
    }

    stateless_default_actions          = local.network_firewall_stateless_default_actions
    stateless_fragment_default_actions = local.network_firewall_stateless_fragment_default_actions

    stateless_rule_group_reference {
      priority     = 10
      resource_arn = aws_networkfirewall_rule_group.stateless[0].arn
    }
  }
}

resource "aws_networkfirewall_firewall" "this" {
  count = local.network_firewall_enabled ? 1 : 0

  name                = "${local.name_prefix}-${var.region_key}-firewall"
  firewall_policy_arn = aws_networkfirewall_firewall_policy.this[0].arn
  vpc_id              = aws_vpc.this[local.network_firewall_inspection_vpc].id

  dynamic "subnet_mapping" {
    for_each = local.network_firewall_subnet_ids
    content {
      subnet_id = subnet_mapping.value
    }
  }

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-network-firewall"
  })
}

resource "aws_cloudwatch_log_group" "network_firewall" {
  count = local.network_firewall_enabled ? 1 : 0

  name              = "/aws/network-firewall/${local.name_prefix}-${var.region_key}"
  retention_in_days = 14

  tags = merge(var.default_tags, {
    Name = "${local.name_prefix}-${var.region_key}-network-firewall"
  })
}

resource "aws_networkfirewall_logging_configuration" "this" {
  count = local.network_firewall_enabled ? 1 : 0

  firewall_arn = aws_networkfirewall_firewall.this[0].arn

  logging_configuration {
    log_destination_config {
      log_destination = {
        logGroup = aws_cloudwatch_log_group.network_firewall[0].name
      }
      log_destination_type = "CloudWatchLogs"
      log_type             = "ALERT"
    }

    log_destination_config {
      log_destination = {
        logGroup = aws_cloudwatch_log_group.network_firewall[0].name
      }
      log_destination_type = "CloudWatchLogs"
      log_type             = "FLOW"
    }
  }
}

resource "aws_security_group" "extra" {
  for_each = local.extra_security_group_map

  name        = "${local.name_prefix}-${var.region_key}-${each.key}-sg"
  description = trimspace(coalesce(try(each.value.description, null), "Skyforge ${each.key} security group"))
  vpc_id      = aws_vpc.this[each.value.vpc].id

  dynamic "ingress" {
    for_each = try(each.value.ingress, [])
    content {
      description      = try(ingress.value.description, null)
      protocol         = ingress.value.protocol
      from_port        = ingress.value.from_port
      to_port          = ingress.value.to_port
      cidr_blocks      = try(ingress.value.cidr_blocks, null)
      ipv6_cidr_blocks = try(ingress.value.ipv6_cidr_blocks, null)
    }
  }

  dynamic "egress" {
    for_each = try(each.value.egress, [])
    content {
      description      = try(egress.value.description, null)
      protocol         = egress.value.protocol
      from_port        = egress.value.from_port
      to_port          = egress.value.to_port
      cidr_blocks      = try(egress.value.cidr_blocks, null)
      ipv6_cidr_blocks = try(egress.value.ipv6_cidr_blocks, null)
    }
  }

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-sg"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "security"
  })
}

resource "aws_security_group" "interface_endpoints" {
  for_each = local.interface_endpoint_map

  name        = "${local.name_prefix}-${var.region_key}-${each.key}-vpce-sg"
  description = "Security group for VPC interface endpoint ${each.key}"
  vpc_id      = aws_vpc.this[each.value.vpc].id

  ingress {
    description = "Allow HTTPS from VPC"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = local.interface_endpoint_allowed_cidrs[each.key]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-vpce-sg"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "vpc-endpoint"
  })
}

resource "aws_vpc_endpoint" "interface" {
  for_each = local.interface_endpoint_map

  vpc_id              = aws_vpc.this[each.value.vpc].id
  service_name        = local.interface_endpoint_service_names[each.key]
  vpc_endpoint_type   = "Interface"
  subnet_ids          = local.interface_endpoint_subnets[each.key]
  security_group_ids  = [aws_security_group.interface_endpoints[each.key].id]
  private_dns_enabled = try(each.value.private_dns_enabled, true)

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-vpce"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "vpc-endpoint"
  })

  lifecycle {
    precondition {
      condition     = length(local.interface_endpoint_subnets[each.key]) > 0
      error_message = "No subnets resolved for interface endpoint ${each.key}."
    }
  }
}

resource "aws_eip" "nat" {
  for_each = {
    for name, subnets in local.nat_gateway_public_subnet_map : name => subnets if length(subnets) > 0
  }

  domain = "vpc"

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-nat-eip"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nat-gateway"
  })
}

resource "aws_nat_gateway" "this" {
  for_each = aws_eip.nat

  allocation_id = each.value.id
  subnet_id     = local.nat_gateway_public_subnet_map[each.key][0]

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-nat"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nat-gateway"
  })

  lifecycle {
    precondition {
      condition     = length(local.nat_gateway_public_subnet_map[each.key]) > 0
      error_message = "No public subnet available for NAT gateway ${each.key}."
    }
  }
}

resource "aws_route_table" "nat_private" {
  for_each = {
    for key, cfg in local.nat_gateway_private_subnet_map : key => cfg if length(cfg.subnet_ids) > 0
  }

  vpc_id = aws_vpc.this[local.nat_gateway_definition_map[each.value.nat_name].vpc].id

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-rt"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nat-route"
  })
}

resource "aws_route" "nat_private_default" {
  for_each = aws_route_table.nat_private

  route_table_id         = each.value.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.this[replace(each.key, "__.*$", "")].id
}

resource "aws_route_table_association" "nat_private" {
  for_each = local.nat_gateway_private_association_map

  subnet_id      = each.value.subnet_id
  route_table_id = aws_route_table.nat_private[each.value.route_key].id
}

resource "aws_vpc_peering_connection" "this" {
  for_each = local.vpc_peering_map

  vpc_id      = aws_vpc.this[each.value.vpc_a].id
  peer_vpc_id = aws_vpc.this[each.value.vpc_b].id
  auto_accept = true
  peer_region = try(each.value.peer_region, null)
  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-peering"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "vpc-peering"
  })
}

resource "aws_route" "vpc_peering_a" {
  for_each = local.vpc_peering_map

  route_table_id            = aws_vpc.this[each.value.vpc_a].main_route_table_id
  destination_cidr_block    = local.vpc_map[each.value.vpc_b].cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.this[each.key].id
}

resource "aws_route" "vpc_peering_b" {
  for_each = local.vpc_peering_map

  route_table_id            = aws_vpc.this[each.value.vpc_b].main_route_table_id
  destination_cidr_block    = local.vpc_map[each.value.vpc_a].cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.this[each.key].id
}

resource "aws_ec2_managed_prefix_list" "this" {
  for_each = local.managed_prefix_list_map

  name           = "${local.name_prefix}-${var.region_key}-${each.key}-pl"
  address_family = "IPv4"
  max_entries    = try(each.value.max_entries, length(each.value.entries))

  dynamic "entry" {
    for_each = try(each.value.entries, [])
    content {
      cidr        = entry.value.cidr
      description = try(entry.value.description, null)
    }
  }

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-pl"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "managed-prefix-list"
  })
}

resource "aws_network_acl" "extra" {
  for_each = local.extra_network_acl_map

  vpc_id = aws_vpc.this[each.value.vpc].id

  tags = merge(var.default_tags, {
    Name           = "${local.name_prefix}-${var.region_key}-${each.key}-acl"
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "security"
  })
}

locals {
  network_acl_ingress_rule_pairs_static = flatten([
    for acl in local.extra_network_acls : [
      for rule in try(acl.ingress, []) : {
        acl_name = acl.name
        rule_no  = rule.rule_no
        rule     = rule
      }
    ]
  ])
  network_acl_egress_rule_pairs_static = flatten([
    for acl in local.extra_network_acls : [
      for rule in try(acl.egress, []) : {
        acl_name = acl.name
        rule_no  = rule.rule_no
        rule     = rule
      }
    ]
  ])
  network_acl_association_pairs_static = flatten([
    for acl in local.extra_network_acls : [
      for subnet_key, subnet in local.subnet_map : {
        acl_name   = acl.name
        subnet_key = subnet_key
      }
      if subnet.vpc_name == acl.vpc && contains(try(acl.subnet_tiers, []), subnet.tier)
    ]
  ])
}

resource "aws_network_acl_rule" "extra_ingress" {
  for_each = {
    for pair in local.network_acl_ingress_rule_pairs_static :
    "${pair.acl_name}-${pair.rule_no}" => pair
  }

  network_acl_id  = aws_network_acl.extra[each.value.acl_name].id
  rule_number     = each.value.rule.rule_no
  egress          = false
  protocol        = lookup(local.acl_protocol_lookup, lower(each.value.rule.protocol), each.value.rule.protocol)
  rule_action     = lower(each.value.rule.action)
  cidr_block      = try(each.value.rule.cidr, null)
  ipv6_cidr_block = try(each.value.rule.ipv6_cidr, null)
  from_port       = each.value.rule.from_port
  to_port         = each.value.rule.to_port
}

resource "aws_network_acl_rule" "extra_egress" {
  for_each = {
    for pair in local.network_acl_egress_rule_pairs_static :
    "${pair.acl_name}-${pair.rule_no}" => pair
  }

  network_acl_id  = aws_network_acl.extra[each.value.acl_name].id
  rule_number     = each.value.rule.rule_no
  egress          = true
  protocol        = lookup(local.acl_protocol_lookup, lower(each.value.rule.protocol), each.value.rule.protocol)
  rule_action     = lower(each.value.rule.action)
  cidr_block      = try(each.value.rule.cidr, null)
  ipv6_cidr_block = try(each.value.rule.ipv6_cidr, null)
  from_port       = each.value.rule.from_port
  to_port         = each.value.rule.to_port
}

resource "aws_network_acl_association" "extra" {
  for_each = {
    for pair in local.network_acl_association_pairs_static :
    "${pair.acl_name}-${pair.subnet_key}" => pair
  }

  network_acl_id = aws_network_acl.extra[each.value.acl_name].id
  subnet_id      = aws_subnet.this[each.value.subnet_key].id
}

module "app_stack" {
  count = local.app_stack_enabled ? 1 : 0

  source = "./app_stack"

  providers = {
    aws = aws
  }

  region_key          = var.region_key
  config              = local.app_stack_config
  default_tags        = var.default_tags
  frontend_vpc_id     = local.app_frontend_vpc_id
  app_vpc_id          = local.app_app_vpc_id
  data_vpc_id         = local.app_data_vpc_id
  frontend_subnet_ids = local.app_frontend_subnets
  app_subnet_ids      = local.app_app_subnets
  data_subnet_ids     = local.app_data_subnets
  resource_suffix     = var.resource_suffix
}

module "fortinet_firewalls" {
  count = local.fortinet_enabled ? 1 : 0

  source = "./firewall_pair"

  providers = {
    aws = aws
  }

  name_prefix          = format("%s-%s", try(local.fortinet_config.name_prefix, "${local.name_prefix}-fortinet"), var.region_key)
  vpc_id               = aws_vpc.this[local.inspection_vpc_name].id
  subnet_ids           = local.inspection_subnet_ids
  ami_id               = try(local.fortinet_config.ami_id, "")
  instance_type        = try(local.fortinet_config.instance_type, "c6i.large")
  iam_instance_profile = try(local.fortinet_config.iam_instance_profile, null)
  user_data            = try(local.fortinet_config.user_data, null)
  tags = merge(var.default_tags, {
    Vendor         = "Fortinet"
    SkyforgeRole   = "security"
    SkyforgeRegion = var.region_key
  })
}

module "checkpoint_firewalls" {
  count = local.checkpoint_enabled ? 1 : 0

  source = "./firewall_pair"

  providers = {
    aws = aws
  }

  name_prefix          = format("%s-%s", try(local.checkpoint_config.name_prefix, "${local.name_prefix}-checkpoint"), var.region_key)
  vpc_id               = aws_vpc.this[local.inspection_vpc_name].id
  subnet_ids           = local.inspection_subnet_ids
  ami_id               = try(local.checkpoint_config.ami_id, "")
  instance_type        = try(local.checkpoint_config.instance_type, "c6i.large")
  iam_instance_profile = try(local.checkpoint_config.iam_instance_profile, null)
  user_data            = try(local.checkpoint_config.user_data, null)
  tags = merge(var.default_tags, {
    Vendor         = "CheckPoint"
    SkyforgeRole   = "security"
    SkyforgeRegion = var.region_key
  })
}

locals {
  vpn_endpoints = {
    region             = var.region_key
    cidr               = var.region_config.cidr_block
    ipv6               = var.region_config.ipv6_prefix
    transit_gateway_id = try(aws_ec2_transit_gateway.this[0].id, null)
    spoke_vpcs = {
      for name, vpc in aws_vpc.this :
      name => {
        id         = vpc.id
        cidr_block = local.vpc_map[name].cidr_block
        subnets = [
          for key, subnet in aws_subnet.this :
          {
            id   = subnet.id
            tier = local.subnet_map[key].tier
            az   = local.subnet_map[key].az
          } if local.subnet_map[key].vpc_name == name
        ]
      }
    }
    gateway_load_balancer = length(module.paloalto_gwlb) > 0 ? {
      arn                 = module.paloalto_gwlb[0].gwlb_arn
      target_group_arn    = module.paloalto_gwlb[0].target_group_arn
      endpoint_service_id = module.paloalto_gwlb[0].endpoint_service_id
    } : null
    transit_gateway_connect = local.transit_gateway_connect_metadata
  }
}

resource "random_password" "vnf_tunnel1_psk" {
  for_each = local.aws_vnf_link_map

  length  = 32
  special = false
}

resource "random_password" "vnf_tunnel2_psk" {
  for_each = local.aws_vnf_link_map

  length  = 32
  special = false
}

resource "aws_customer_gateway" "vnf" {
  for_each = local.aws_vnf_link_map

  bgp_asn    = each.value.bgp_asn
  ip_address = each.value.customer_gateway_ipv4
  type       = "ipsec.1"

  tags = merge(var.default_tags, {
    Name         = "${local.name_prefix}-${var.region_key}-${each.key}-cgw"
    SkyforgeSite = each.key
    SkyforgeRole = "vpn-hub"
  })
}

resource "aws_vpn_connection" "vnf" {
  for_each = local.aws_vnf_link_map

  transit_gateway_id  = aws_ec2_transit_gateway.this[0].id
  customer_gateway_id = aws_customer_gateway.vnf[each.key].id
  type                = "ipsec.1"
  static_routes_only  = false

  tunnel1_preshared_key = random_password.vnf_tunnel1_psk[each.key].result
  tunnel2_preshared_key = random_password.vnf_tunnel2_psk[each.key].result

  tags = merge(var.default_tags, {
    Name         = "${local.name_prefix}-${var.region_key}-${each.key}-vpn"
    SkyforgeSite = each.key
    SkyforgeRole = "vpn-hub"
  })

  depends_on = [aws_ec2_transit_gateway.this]
}
