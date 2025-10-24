output "ha_vpn_gateway_self_link" {
  description = "Self link of the HA VPN gateway (if created)."
  value       = try(google_compute_ha_vpn_gateway.this[0].self_link, null)
}

output "network_ids" {
  description = "Map of network IDs by logical name."
  value = {
    for name, network in google_compute_network.this :
    name => network.id
  }
}

output "vpn_manifest" {
  description = "Structured manifest describing GCP VPN and network layout."
  value       = local.vpn_manifest
}

output "vpn_connections" {
  description = "GCP VPN connection metadata keyed by VNF site."
  value = {
    for site, tunnel in google_compute_vpn_tunnel.vnf :
    site => {
      id               = tunnel.id
      shared_secret    = random_password.vnf_shared_secret[site].result
      router_interface = google_compute_router_interface.vnf[site].name
      router_peer      = google_compute_router_peer.vnf[site].name
      peer_ip          = google_compute_router_peer.vnf[site].peer_ip_address
      local_ip         = google_compute_router_peer.vnf[site].ip_address
      inside_cidr      = local.gcp_vnf_link_ranges[site]
      external_gateway = google_compute_external_vpn_gateway.vnf[site].id
    }
  }
}

output "workloads" {
  description = "Metadata for optional GCP workloads."
  value       = length(module.workloads) > 0 ? module.workloads[0].metadata : null
}

output "ha_vpn_gateway_external_ips" {
  description = "External IP addresses allocated to the HA VPN gateway interfaces."
  value = try([
    for iface in google_compute_ha_vpn_gateway.this[0].vpn_interfaces :
    iface.ip_address
  ], [])
}

output "default_router" {
  description = "Metadata for the default Cloud Router associated with the region."
  value = local.default_vpc_key != null ? {
    name   = google_compute_router.this[local.default_vpc_key].name
    region = var.region_config.region
  } : null
}

output "checkpoint_firewall" {
  description = "Metadata for the optional Check Point firewall instance."
  value = length(google_compute_instance.checkpoint) > 0 ? {
    instance_id    = google_compute_instance.checkpoint[0].id
    self_link      = google_compute_instance.checkpoint[0].self_link
    private_ip     = google_compute_instance.checkpoint[0].network_interface[0].network_ip
    zone           = google_compute_instance.checkpoint[0].zone
    admin_username = local.checkpoint_admin_username
    admin_password = local.checkpoint_admin_password
  } : null
  sensitive = true
}
