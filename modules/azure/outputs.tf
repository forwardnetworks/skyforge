output "virtual_network_ids" {
  description = "Map of Azure virtual networks by logical name."
  value = {
    for name, vnet in azurerm_virtual_network.this :
    name => vnet.id
  }
}

output "virtual_wan_hub_id" {
  description = "Virtual hub resource ID if created."
  value       = try(azurerm_virtual_hub.this[0].id, null)
}

output "vpn_manifest" {
  description = "Structured output for VPN/connectivity orchestration."
  value       = local.vpn_manifest
}

output "vpn_gateway_id" {
  description = "Azure VPN gateway resource ID when virtual WAN is enabled."
  value       = try(azurerm_vpn_gateway.this[0].id, null)
}

output "virtual_hub_connections" {
  description = "Virtual hub connection IDs by VNet name."
  value = {
    for name, conn in azurerm_virtual_hub_connection.this :
    name => conn.id
  }
}

output "vpn_connections" {
  description = "Azure VPN connection metadata keyed by VNF site."
  value = {
    for site, conn in azurerm_vpn_gateway_connection.vnf :
    site => {
      id             = conn.id
      vpn_site_id    = azurerm_vpn_site.vnf[site].id
      vpn_gateway_id = azurerm_vpn_gateway.this[0].id
      shared_key     = random_password.vnf_link_psk[site].result
    }
  }
}

output "vpn_gateway_public_ips" {
  description = "Azure VPN gateway public tunnel IPs."
  value = try([
    for config in azurerm_vpn_gateway.this[0].ip_configuration :
    config.public_ip_address
  ], [])
}

output "resource_group_name" {
  description = "Resource group hosting the Azure regional deployment."
  value       = local.resource_group_name
}

output "location" {
  description = "Azure location for the regional resources."
  value       = var.region_config.location
}

output "virtual_wan_id" {
  description = "Virtual WAN resource ID when enabled."
  value       = try(azurerm_virtual_wan.this[0].id, null)
}

output "network_watcher_id" {
  description = "Network watcher resource ID when deployed."
  value       = try(azurerm_network_watcher.this[0].id, null)
}

output "nva_route_tables" {
  description = "Route table IDs for NVA chain by VNet name."
  value = {
    for name, rt in azurerm_route_table.nva :
    name => rt.id
  }
}

output "workloads" {
  description = "Metadata for Azure workloads when configured."
  value       = length(module.workloads) > 0 ? module.workloads[0].metadata : null
}

output "firewall_id" {
  description = "Azure Firewall resource ID when deployed."
  value       = try(azurerm_firewall.this[0].id, null)
}

output "firewall_policy_id" {
  description = "Azure Firewall policy resource ID when deployed."
  value       = try(azurerm_firewall_policy.this[0].id, null)
}

output "firewall_public_ip" {
  description = "Azure Firewall public IP address when deployed."
  value       = try(azurerm_public_ip.firewall[0].ip_address, null)
}

output "asa_nva" {
  description = "Cisco ASA virtual appliance metadata when deployed."
  value       = local.asa_metadata
  sensitive   = true
}
