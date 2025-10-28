locals {
  name_prefix         = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
  resource_group_name = coalesce(var.resource_group_name, "rg-${local.name_prefix}-${var.region_key}")
}

resource "azurerm_resource_group" "region" {
  count    = var.resource_group_name == null ? 1 : 0
  name     = local.resource_group_name
  location = var.region_config.location

  tags = merge(var.default_tags, {
    Name           = local.resource_group_name
    SkyforgeRegion = var.region_key
  })
}

locals {
  network_watcher_enabled = try(var.region_config.enable_network_watcher, true)
}

resource "azurerm_network_watcher" "this" {
  count = local.network_watcher_enabled ? 1 : 0

  name                = "nw-${local.name_prefix}-${var.region_key}"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name

  tags = merge(var.default_tags, {
    Name           = "nw-${local.name_prefix}-${var.region_key}"
    SkyforgeRegion = var.region_key
  })
}

locals {
  vnet_map = var.region_config.vnets

  base_subnet_blueprints = flatten([
    for vnet_name, vnet in local.vnet_map : [
      for tier_index, tier in vnet.tiers : [
        for offset in range(vnet.subnets_per_tier) : {
          key            = "${vnet_name}-${tier}-${offset}"
          vnet_name      = vnet_name
          tier           = tier
          address_prefix = cidrsubnet(vnet.address_space[0], 4, tier_index * vnet.subnets_per_tier + offset)
          offset         = offset
          custom_name    = null
        }
      ]
    ]
  ])

  firewall_vnet_name = var.region_config.enable_firewall ? (
    contains(keys(local.vnet_map), "inspection") ? "inspection" : element(keys(local.vnet_map), 0)
  ) : null

  firewall_subnet_index = local.firewall_vnet_name == null ? null : length(local.vnet_map[local.firewall_vnet_name].tiers) * local.vnet_map[local.firewall_vnet_name].subnets_per_tier

  firewall_subnet_blueprints = local.firewall_vnet_name == null ? [] : [
    {
      key            = "${local.firewall_vnet_name}-AzureFirewallSubnet"
      vnet_name      = local.firewall_vnet_name
      tier           = "firewall"
      address_prefix = cidrsubnet(local.vnet_map[local.firewall_vnet_name].address_space[0], 4, local.firewall_subnet_index)
      offset         = 0
      custom_name    = "AzureFirewallSubnet"
    }
  ]

  subnet_blueprints = concat(local.base_subnet_blueprints, local.firewall_subnet_blueprints)

  subnet_map = { for blueprint in local.subnet_blueprints : blueprint.key => blueprint }

  firewall_subnet_key = local.firewall_vnet_name == null ? null : "${local.firewall_vnet_name}-AzureFirewallSubnet"
}

locals {
  subnet_ids_by_tier = {
    for key, subnet in azurerm_subnet.this :
    "${local.subnet_map[key].vnet_name}.${local.subnet_map[key].tier}" => subnet.id...
  }

  subnet_id_map = {
    for tier_key, ids in local.subnet_ids_by_tier :
    tier_key => ids[0]
  }

  vnet_id_map = {
    for name, vnet in azurerm_virtual_network.this :
    name => vnet.id
  }
}

locals {
  nat_gateway_definitions = try(var.region_config.nat_gateways, [])
  nat_gateway_map         = { for nat in local.nat_gateway_definitions : nat.name => nat }
  nat_gateway_private_subnet_keys = {
    for nat in local.nat_gateway_definitions :
    nat.name => [
      for key, subnet in local.subnet_map :
      key if subnet.vnet_name == nat.vnet_name && contains(nat.private_subnet_tiers, subnet.tier)
    ]
  }
  nat_gateway_private_association_static = flatten([
    for nat in local.nat_gateway_definitions : [
      for subnet_key in lookup(local.nat_gateway_private_subnet_keys, nat.name, []) : {
        key        = "${nat.name}-${subnet_key}"
        nat_name   = nat.name
        subnet_key = subnet_key
      }
    ]
  ])
  asa_config                   = try(var.region_config.asa_nva, null)
  asa_enabled                  = local.asa_config != null && try(local.asa_config.enable, true)
  asa_subnet_id                = local.asa_enabled ? lookup(local.subnet_id_map, "${local.asa_config.vnet_name}.${local.asa_config.subnet_tier}", null) : null
  vnet_peering_definitions     = try(var.region_config.vnet_peerings, [])
  vnet_peering_map             = { for peer in local.vnet_peering_definitions : peer.name => peer }
  private_endpoint_definitions = try(var.region_config.private_endpoints, [])
  private_endpoint_map         = { for pe in local.private_endpoint_definitions : pe.name => pe }
  private_endpoint_subnets = {
    for pe in local.private_endpoint_definitions :
    pe.name => lookup(local.subnet_id_map, "${pe.vnet_name}.${pe.subnet_tier}", null)
  }
}

resource "azurerm_virtual_network" "this" {
  for_each = local.vnet_map

  name                = "vnet-${var.region_key}-${each.key}"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  address_space       = each.value.address_space

  tags = merge(var.default_tags, {
    Name           = "vnet-${var.region_key}-${each.key}"
    SkyforgeRegion = var.region_key
    SkyforgeVNet   = each.key
  })
}

resource "azurerm_subnet" "this" {
  for_each = local.subnet_map

  name                 = each.value.custom_name != null ? each.value.custom_name : format("subnet-%s-%s-%02d", each.value.vnet_name, each.value.tier, each.value.offset)
  resource_group_name  = local.resource_group_name
  virtual_network_name = azurerm_virtual_network.this[each.value.vnet_name].name
  address_prefixes     = [each.value.address_prefix]

  depends_on = [azurerm_virtual_network.this]
}

resource "azurerm_public_ip" "nat" {
  for_each = local.nat_gateway_map

  name                = format("pip-%s-%s-%s-nat", local.name_prefix, var.region_key, each.key)
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = merge(var.default_tags, {
    Name           = format("pip-%s-%s-%s-nat", local.name_prefix, var.region_key, each.key)
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nat-gateway"
  })
}

resource "azurerm_nat_gateway" "this" {
  for_each = local.nat_gateway_map

  name                    = format("nat-%s-%s-%s", local.name_prefix, var.region_key, each.key)
  location                = var.region_config.location
  resource_group_name     = local.resource_group_name
  sku_name                = "Standard"
  idle_timeout_in_minutes = 10

  tags = merge(var.default_tags, {
    Name           = format("nat-%s-%s-%s", local.name_prefix, var.region_key, each.key)
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nat-gateway"
  })
}

resource "azurerm_nat_gateway_public_ip_association" "this" {
  for_each = local.nat_gateway_map

  nat_gateway_id       = azurerm_nat_gateway.this[each.key].id
  public_ip_address_id = azurerm_public_ip.nat[each.key].id
}

resource "azurerm_subnet_nat_gateway_association" "this" {
  for_each = {
    for assoc in local.nat_gateway_private_association_static :
    assoc.key => assoc
  }

  subnet_id      = azurerm_subnet.this[each.value.subnet_key].id
  nat_gateway_id = azurerm_nat_gateway.this[each.value.nat_name].id
}

resource "azurerm_public_ip" "firewall" {
  count               = var.region_config.enable_firewall ? 1 : 0
  name                = "pip-${local.name_prefix}-${var.region_key}-azfw"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = merge(var.default_tags, {
    Name = "pip-${local.name_prefix}-${var.region_key}-azfw"
  })
}

resource "azurerm_virtual_wan" "this" {
  count = var.region_config.enable_virtual_wan ? 1 : 0

  name                = "vwan-${local.name_prefix}-${var.region_key}"
  resource_group_name = local.resource_group_name
  location            = var.region_config.location

  tags = merge(var.default_tags, {
    Name = "vwan-${local.name_prefix}-${var.region_key}"
  })
}

resource "azurerm_virtual_hub" "this" {
  count = var.region_config.enable_virtual_wan ? 1 : 0

  name                = "vhub-${local.name_prefix}-${var.region_key}"
  resource_group_name = local.resource_group_name
  location            = var.region_config.location
  address_prefix      = coalesce(try(var.region_config.virtual_hub_address_prefix, null), cidrsubnet(var.region_config.cidr_block, 4, 15))
  sku                 = "Standard"
  virtual_wan_id      = azurerm_virtual_wan.this[0].id
}

resource "azurerm_vpn_gateway" "this" {
  count = var.region_config.enable_virtual_wan ? 1 : 0

  name                = "vpngw-${local.name_prefix}-${var.region_key}"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  virtual_hub_id      = azurerm_virtual_hub.this[0].id
  scale_unit          = 1

  bgp_settings {
    asn         = 65515
    peer_weight = 0
  }

  tags = merge(var.default_tags, {
    Name = "vpngw-${local.name_prefix}-${var.region_key}"
  })
}

resource "azurerm_virtual_hub_connection" "this" {
  for_each = var.region_config.enable_virtual_wan ? azurerm_virtual_network.this : {}

  name                      = "vhc-${local.name_prefix}-${var.region_key}-${each.key}"
  virtual_hub_id            = azurerm_virtual_hub.this[0].id
  remote_virtual_network_id = each.value.id
  internet_security_enabled = false

  routing {
    associated_route_table_id = azurerm_virtual_hub.this[0].default_route_table_id

    propagated_route_table {
      route_table_ids = [azurerm_virtual_hub.this[0].default_route_table_id]
    }
  }
}

resource "azurerm_firewall_policy" "this" {
  count               = var.region_config.enable_firewall ? 1 : 0
  name                = "fp-${local.name_prefix}-${var.region_key}"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  sku                 = "Standard"
  tags = merge(var.default_tags, {
    Name = "fp-${local.name_prefix}-${var.region_key}"
  })
}

locals {
  firewall_policy_config                 = var.region_config.enable_firewall ? try(var.region_config.firewall_policy, null) : null
  firewall_network_collections_input     = local.firewall_policy_config != null ? try(local.firewall_policy_config.network_rule_collections, []) : []
  firewall_application_collections_input = local.firewall_policy_config != null ? try(local.firewall_policy_config.application_rule_collections, []) : []
  firewall_nat_collections_input         = local.firewall_policy_config != null ? try(local.firewall_policy_config.nat_rule_collections, []) : []

  firewall_network_collections = [
    for collection in local.firewall_network_collections_input : {
      name     = collection.name
      priority = collection.priority
      action   = lower(trimspace(collection.action)) == "deny" ? "Deny" : "Allow"
      rules = [
        for rule in collection.rules : {
          name                  = rule.name
          description           = try(rule.description, null)
          source_addresses      = try(rule.source_addresses, ["*"])
          destination_addresses = try(rule.destination_addresses, [])
          destination_fqdns     = try(rule.destination_fqdns, [])
          destination_ports     = try(rule.destination_ports, [])
          protocols             = [for protocol in try(rule.protocols, ["Any"]) : upper(protocol)]
        }
      ]
    }
  ]

  firewall_default_network_collection = {
    name     = "${local.name_prefix}-net-allow"
    priority = 200
    action   = "Allow"
    rules = [
      {
        name                  = "allow-internal"
        description           = "Allow egress from regional CIDR to essential services"
        source_addresses      = [var.region_config.cidr_block]
        destination_addresses = ["0.0.0.0/0"]
        destination_fqdns     = []
        destination_ports     = ["53", "443"]
        protocols             = ["TCP", "UDP"]
      }
    ]
  }

  firewall_network_collections_effective = length(local.firewall_network_collections) > 0 ? local.firewall_network_collections : [local.firewall_default_network_collection]

  firewall_application_collections = [
    for collection in local.firewall_application_collections_input : {
      name     = collection.name
      priority = collection.priority
      action   = lower(trimspace(collection.action)) == "deny" ? "Deny" : "Allow"
      rules = [
        for rule in collection.rules : {
          name                  = rule.name
          description           = try(rule.description, null)
          source_addresses      = try(rule.source_addresses, ["*"])
          destination_addresses = try(rule.destination_addresses, [])
          destination_fqdn_tags = try(rule.destination_fqdn_tags, [])
          destination_fqdns     = try(rule.destination_fqdns, [])
          destination_urls      = try(rule.destination_urls, [])
          source_ip_groups      = try(rule.source_ip_groups, [])
          web_categories        = try(rule.web_categories, [])
          protocols = [
            for proto in try(rule.protocols, []) : {
              type = upper(proto.type)
              port = proto.port
            }
          ]
        }
      ]
    }
  ]

  firewall_nat_collections = [
    for collection in local.firewall_nat_collections_input : {
      name     = collection.name
      priority = collection.priority
      action   = lower(trimspace(collection.action)) == "snat" ? "Snat" : "Dnat"
      rules = [
        for rule in collection.rules : {
          name                = rule.name
          description         = try(rule.description, null)
          source_addresses    = try(rule.source_addresses, ["*"])
          destination_address = try(rule.destination_address, try(rule.destination_addresses[0], null))
          destination_ports   = try(rule.destination_ports, [])
          protocols           = [for protocol in try(rule.protocols, ["TCP"]) : upper(protocol)]
          translated_address  = try(rule.translated_address, null)
          translated_fqdn     = try(rule.translated_fqdn, null)
          translated_port     = try(tonumber(rule.translated_port), null)
          source_ip_groups    = try(rule.source_ip_groups, [])
        }
      ]
    }
  ]
}

resource "azurerm_firewall_policy_rule_collection_group" "this" {
  count              = var.region_config.enable_firewall ? 1 : 0
  name               = "rcg-${local.name_prefix}-${var.region_key}"
  priority           = 100
  firewall_policy_id = azurerm_firewall_policy.this[0].id

  dynamic "network_rule_collection" {
    for_each = local.firewall_network_collections_effective
    content {
      name     = network_rule_collection.value.name
      priority = network_rule_collection.value.priority
      action   = network_rule_collection.value.action

      dynamic "rule" {
        for_each = network_rule_collection.value.rules
        content {
          name                  = rule.value.name
          source_addresses      = rule.value.source_addresses
          destination_addresses = rule.value.destination_addresses
          destination_fqdns     = rule.value.destination_fqdns
          destination_ports     = rule.value.destination_ports
          protocols             = rule.value.protocols
          description           = rule.value.description
        }
      }
    }
  }

  dynamic "application_rule_collection" {
    for_each = local.firewall_application_collections
    content {
      name     = application_rule_collection.value.name
      priority = application_rule_collection.value.priority
      action   = application_rule_collection.value.action

      dynamic "rule" {
        for_each = application_rule_collection.value.rules
        content {
          name                  = rule.value.name
          source_addresses      = rule.value.source_addresses
          source_ip_groups      = rule.value.source_ip_groups
          destination_addresses = rule.value.destination_addresses
          destination_fqdn_tags = rule.value.destination_fqdn_tags
          destination_fqdns     = rule.value.destination_fqdns
          destination_urls      = rule.value.destination_urls
          web_categories        = rule.value.web_categories
          description           = rule.value.description

          dynamic "protocols" {
            for_each = rule.value.protocols
            content {
              type = protocols.value.type
              port = protocols.value.port
            }
          }
        }
      }
    }
  }

  dynamic "nat_rule_collection" {
    for_each = local.firewall_nat_collections
    content {
      name     = nat_rule_collection.value.name
      priority = nat_rule_collection.value.priority
      action   = nat_rule_collection.value.action

      dynamic "rule" {
        for_each = nat_rule_collection.value.rules
        content {
          name                = rule.value.name
          source_addresses    = rule.value.source_addresses
          source_ip_groups    = rule.value.source_ip_groups
          destination_address = rule.value.destination_address
          destination_ports   = rule.value.destination_ports
          protocols           = rule.value.protocols
          translated_address  = rule.value.translated_address
          translated_fqdn     = rule.value.translated_fqdn
          translated_port     = rule.value.translated_port
          description         = rule.value.description
        }
      }
    }
  }
}

resource "random_password" "asa_admin" {
  count = local.asa_enabled && try(local.asa_config.admin_password, null) == null ? 1 : 0

  length  = 16
  special = false
}

resource "azurerm_public_ip" "asa" {
  count = local.asa_enabled ? 1 : 0

  name                = format("pip-%s-%s-asa", local.name_prefix, var.region_key)
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = merge(var.default_tags, {
    Name           = format("pip-%s-%s-asa", local.name_prefix, var.region_key)
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nva"
  })
}

resource "azurerm_network_interface" "asa" {
  count = local.asa_enabled ? 1 : 0

  name                = format("nic-%s-%s-asa", local.name_prefix, var.region_key)
  location            = var.region_config.location
  resource_group_name = local.resource_group_name

  ip_configuration {
    name                          = "primary"
    subnet_id                     = local.asa_subnet_id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.asa[0].id
  }

  tags = merge(var.default_tags, {
    Name           = format("nic-%s-%s-asa", local.name_prefix, var.region_key)
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nva"
  })

  lifecycle {
    precondition {
      condition     = local.asa_subnet_id != null
      error_message = "ASA NVA subnet lookup failed. Ensure asa_nva.vnet_name and subnet_tier are valid."
    }
  }
}

resource "azurerm_linux_virtual_machine" "asa" {
  count = local.asa_enabled ? 1 : 0

  name                = format("vm-%s-%s-asa", local.name_prefix, var.region_key)
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  size                = coalesce(try(local.asa_config.vm_size, null), "Standard_D4s_v5")
  admin_username      = coalesce(try(local.asa_config.admin_username, null), "asaadmin")
  admin_password      = coalesce(try(local.asa_config.admin_password, null), random_password.asa_admin[0].result)
  network_interface_ids = [
    azurerm_network_interface.asa[0].id
  ]

  disable_password_authentication = false

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-focal"
    sku       = "20_04-lts"
    version   = "latest"
  }

  os_disk {
    name                 = format("osdisk-%s-%s-asa", local.name_prefix, var.region_key)
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  custom_data = base64encode(<<-EOT
    #!/bin/bash
    echo "Cisco ASA demo NVA placeholder" > /var/log/skyforge-asa.log
  EOT
  )

  tags = merge(var.default_tags, {
    Name           = format("vm-%s-%s-asa", local.name_prefix, var.region_key)
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "nva"
    Vendor         = "Cisco"
  })
}

resource "azurerm_virtual_network_peering" "a_to_b" {
  for_each = local.vnet_peering_map

  name                         = format("peer-%s-%s-to-%s", var.region_key, each.value.vnet_a, each.value.vnet_b)
  resource_group_name          = local.resource_group_name
  virtual_network_name         = azurerm_virtual_network.this[each.value.vnet_a].name
  remote_virtual_network_id    = azurerm_virtual_network.this[each.value.vnet_b].id
  allow_forwarded_traffic      = true
  allow_gateway_transit        = false
  allow_virtual_network_access = true

  lifecycle {
    precondition {
      condition     = contains(keys(local.vnet_id_map), each.value.vnet_a) && contains(keys(local.vnet_id_map), each.value.vnet_b)
      error_message = "VNet peering references unknown VNet."
    }
  }
}

resource "azurerm_virtual_network_peering" "b_to_a" {
  for_each = local.vnet_peering_map

  name                         = format("peer-%s-%s-to-%s-return", var.region_key, each.value.vnet_b, each.value.vnet_a)
  resource_group_name          = local.resource_group_name
  virtual_network_name         = azurerm_virtual_network.this[each.value.vnet_b].name
  remote_virtual_network_id    = azurerm_virtual_network.this[each.value.vnet_a].id
  allow_forwarded_traffic      = true
  allow_gateway_transit        = false
  allow_virtual_network_access = true
}

resource "azurerm_private_endpoint" "this" {
  for_each = {
    for name, cfg in local.private_endpoint_map :
    name => cfg if lookup(local.private_endpoint_subnets, name, null) != null
  }

  name                = format("pep-%s-%s-%s", local.name_prefix, var.region_key, each.key)
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  subnet_id           = local.private_endpoint_subnets[each.key]

  private_service_connection {
    name                           = format("psc-%s", each.key)
    private_connection_resource_id = each.value.resource_id
    is_manual_connection           = try(each.value.is_manual_connection, false)
    subresource_names              = try(each.value.subresource_names, null)
    request_message                = try(each.value.request_message, null)
  }

  dynamic "private_dns_zone_group" {
    for_each = length(try(each.value.private_dns_zone_ids, [])) > 0 ? {
      default = each.value.private_dns_zone_ids
    } : {}
    content {
      name                 = format("pdzg-%s", each.key)
      private_dns_zone_ids = private_dns_zone_group.value
    }
  }

  tags = merge(var.default_tags, {
    Name           = format("pep-%s-%s-%s", local.name_prefix, var.region_key, each.key)
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "private-endpoint"
  })
}

resource "azurerm_firewall" "this" {
  count               = var.region_config.enable_firewall ? 1 : 0
  name                = "azfw-${local.name_prefix}-${var.region_key}"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  sku_name            = "AZFW_VNet"
  sku_tier            = "Standard"
  firewall_policy_id  = azurerm_firewall_policy.this[0].id
  threat_intel_mode   = "Alert"

  ip_configuration {
    name                 = "configuration"
    subnet_id            = azurerm_subnet.this[local.firewall_subnet_key].id
    public_ip_address_id = azurerm_public_ip.firewall[0].id
  }

  tags = merge(var.default_tags, {
    Name = "azfw-${local.name_prefix}-${var.region_key}"
  })
}

locals {
  asa_metadata = local.asa_enabled ? {
    vm_id          = azurerm_linux_virtual_machine.asa[0].id
    nic_id         = azurerm_network_interface.asa[0].id
    public_ip      = azurerm_public_ip.asa[0].ip_address
    private_ip     = azurerm_network_interface.asa[0].ip_configuration[0].private_ip_address
    admin_username = coalesce(try(local.asa_config.admin_username, null), "asaadmin")
    admin_password = coalesce(try(local.asa_config.admin_password, null), random_password.asa_admin[0].result)
  } : null
}

module "workloads" {
  count = try(var.region_config.workloads, null) == null ? 0 : 1

  source              = "./workloads"
  region_key          = var.region_key
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  default_tags        = var.default_tags
  config              = var.region_config.workloads
  subnet_id_map       = local.subnet_id_map
  vnet_id_map         = local.vnet_id_map
  firewall_private_ip = length(azurerm_firewall.this) > 0 ? azurerm_firewall.this[0].ip_configuration[0].private_ip_address : null
  resource_suffix     = var.resource_suffix

  providers = {
    azurerm = azurerm
  }
}

locals {
  nva_chain_config_raw       = try(var.region_config.nva_chain, null)
  nva_chain_enabled          = local.nva_chain_config_raw != null && try(local.nva_chain_config_raw.enable, false)
  nva_route_collections_base = local.nva_chain_enabled ? try(local.nva_chain_config_raw.route_collections, {}) : {}
  nva_route_collections = { for vnet_name, cfg in local.nva_route_collections_base :
    vnet_name => {
      tiers                   = cfg.tiers
      destinations            = try(cfg.destinations, ["0.0.0.0/0"])
      next_hop_type           = try(cfg.next_hop_type, "VirtualAppliance")
      next_hop_ip             = try(cfg.next_hop_ip, null)
      use_firewall_private_ip = try(cfg.use_firewall_private_ip, false)
      disable_bgp_propagation = try(cfg.disable_bgp_propagation, false)
    }
  }
  nva_route_collections_enriched = {
    for vnet_name, cfg in local.nva_route_collections :
    vnet_name => merge(cfg, {
      resolved_next_hop_ip = cfg.next_hop_type == "VirtualAppliance" ? (
        cfg.use_firewall_private_ip && length(azurerm_firewall.this) > 0 ? azurerm_firewall.this[0].ip_configuration[0].private_ip_address : cfg.next_hop_ip
      ) : null
    })
  }
  nva_route_entries = local.nva_chain_enabled ? flatten([
    for vnet_name, cfg in local.nva_route_collections_enriched : [
      for destination in cfg.destinations : {
        key           = "${vnet_name}-${replace(destination, "/", "-")}"
        vnet_name     = vnet_name
        destination   = destination
        next_hop_type = cfg.next_hop_type
        next_hop_ip   = cfg.next_hop_type == "VirtualAppliance" ? cfg.resolved_next_hop_ip : null
      }
    ]
  ]) : []
  nva_route_entry_map = { for entry in local.nva_route_entries : entry.key => entry }
  nva_subnet_associations = local.nva_chain_enabled ? flatten([
    for vnet_name, cfg in local.nva_route_collections_enriched : [
      for tier in cfg.tiers : [
        for key, subnet in azurerm_subnet.this :
        {
          key       = "${vnet_name}-${tier}-${key}"
          vnet_name = vnet_name
          subnet_id = subnet.id
        }
        if local.subnet_map[key].vnet_name == vnet_name && local.subnet_map[key].tier == tier
      ]
    ]
  ]) : []
  nva_subnet_association_map = { for assoc in local.nva_subnet_associations : assoc.key => assoc }
}

resource "azurerm_route_table" "nva" {
  for_each = local.nva_chain_enabled ? local.nva_route_collections_enriched : {}

  name                = "rt-${local.name_prefix}-${var.region_key}-${each.key}-nva"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name

  tags = merge(var.default_tags, {
    Name           = "rt-${local.name_prefix}-${var.region_key}-${each.key}-nva"
    SkyforgeRegion = var.region_key
    SkyforgeVNet   = each.key
  })
}

resource "azurerm_route" "nva" {
  for_each = local.nva_chain_enabled ? local.nva_route_entry_map : {}

  name                   = "rt-${each.value.vnet_name}-${replace(each.value.destination, "/", "-")}"
  resource_group_name    = local.resource_group_name
  route_table_name       = azurerm_route_table.nva[each.value.vnet_name].name
  address_prefix         = each.value.destination
  next_hop_type          = each.value.next_hop_type
  next_hop_in_ip_address = each.value.next_hop_type == "VirtualAppliance" ? each.value.next_hop_ip : null

  lifecycle {
    precondition {
      condition     = each.value.next_hop_type != "VirtualAppliance" || each.value.next_hop_ip != null
      error_message = "VirtualAppliance routes require a next_hop_ip or use_firewall_private_ip=true."
    }
  }

  depends_on = [azurerm_firewall.this]
}

resource "azurerm_subnet_route_table_association" "nva" {
  for_each = local.nva_chain_enabled ? local.nva_subnet_association_map : {}

  subnet_id      = each.value.subnet_id
  route_table_id = azurerm_route_table.nva[each.value.vnet_name].id
}

locals {
  vpn_manifest = {
    region   = var.region_key
    location = var.region_config.location
    cidr     = var.region_config.cidr_block
    hub_id   = try(azurerm_virtual_hub.this[0].id, null)
    vnets = {
      for name, vnet in azurerm_virtual_network.this :
      name => {
        id            = vnet.id
        address_space = vnet.address_space
        subnets = [
          for key, subnet in azurerm_subnet.this :
          {
            id   = subnet.id
            tier = local.subnet_map[key].tier
            cidr = local.subnet_map[key].address_prefix
          } if local.subnet_map[key].vnet_name == name
        ]
      }
    }
    firewall = length(azurerm_firewall.this) > 0 ? {
      id                   = azurerm_firewall.this[0].id
      public_ip_address    = azurerm_public_ip.firewall[0].ip_address
      subnet_id            = azurerm_subnet.this[local.firewall_subnet_key].id
      firewall_vnet        = local.firewall_vnet_name
      firewall_subnet_cidr = local.firewall_vnet_name == null ? null : local.subnet_map[local.firewall_subnet_key].address_prefix
    } : null
    asa = local.asa_metadata
  }
}

locals {
  vpn_gateway_enabled = var.region_config.enable_virtual_wan
  azure_vnf_endpoint_candidates = {
    for link in var.vnf_links :
    link.site => (
      link.preferred_proto == "IPv6"
      ? compact([try(link.customer_gateway_ipv6, null), try(link.customer_gateway_ipv4, null)])
      : compact([try(link.customer_gateway_ipv4, null), try(link.customer_gateway_ipv6, null)])
    )
  }
  azure_vnf_link_map = local.vpn_gateway_enabled ? {
    for link in var.vnf_links :
    link.site => merge(link, {
      endpoint_ip        = length(local.azure_vnf_endpoint_candidates[link.site]) > 0 ? local.azure_vnf_endpoint_candidates[link.site][0] : null
      preferred_peer_ip  = length(local.azure_vnf_endpoint_candidates[link.site]) > 0 ? local.azure_vnf_endpoint_candidates[link.site][0] : null
      alternate_peer_ips = slice(local.azure_vnf_endpoint_candidates[link.site], 1, length(local.azure_vnf_endpoint_candidates[link.site]))
    })
    if length(local.azure_vnf_endpoint_candidates[link.site]) > 0
  } : {}
}

resource "random_password" "vnf_link_psk" {
  for_each = local.azure_vnf_link_map

  length  = 32
  special = false
}

resource "azurerm_vpn_site" "vnf" {
  for_each = local.azure_vnf_link_map

  name                = "vpnsite-${local.name_prefix}-${var.region_key}-${each.key}"
  location            = var.region_config.location
  resource_group_name = local.resource_group_name
  virtual_wan_id      = azurerm_virtual_wan.this[0].id

  address_cidrs = toset(compact([
    try(var.vnf_sites[each.key].ipv4_cidr, null),
    try(var.vnf_sites[each.key].ipv6_prefix, null)
  ]))

  link {
    name       = "link-1"
    ip_address = each.value.endpoint_ip

    bgp {
      asn             = each.value.bgp_asn
      peering_address = coalesce(each.value.preferred_peer_ip, try(each.value.customer_gateway_ipv4, null), try(each.value.customer_gateway_ipv6, null))
    }
  }

  tags = merge(var.default_tags, {
    Name         = "vpnsite-${local.name_prefix}-${var.region_key}-${each.key}"
    SkyforgeSite = each.key
  })
}

resource "azurerm_vpn_gateway_connection" "vnf" {
  for_each = local.azure_vnf_link_map

  name                      = "vpnconn-${local.name_prefix}-${var.region_key}-${each.key}"
  vpn_gateway_id            = azurerm_vpn_gateway.this[0].id
  remote_vpn_site_id        = azurerm_vpn_site.vnf[each.key].id
  internet_security_enabled = false

  vpn_link {
    name             = "link-1"
    vpn_site_link_id = azurerm_vpn_site.vnf[each.key].link[0].id
    protocol         = "IKEv2"
    bgp_enabled      = true
    shared_key       = random_password.vnf_link_psk[each.key].result
  }
}
