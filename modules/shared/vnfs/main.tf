locals {
  site_details = {
    for name, site in var.sites :
    name => merge(site, {
      primary_asn      = 65000 + index(sort(keys(var.sites)), name)
      connect_to_cloud = lookup(site, "connect_to_cloud", true)
      vpn_gateway_ipv6 = try(site.vpn_gateway_ipv6, null)
      vpn_gateway_ipv4 = try(site.vpn_gateway_ipv4, null)
      preferred_gateway_ip = lookup(site, "connect_to_cloud", true) ? (
        length(compact([try(site.vpn_gateway_ipv6, null), try(site.vpn_gateway_ipv4, null)])) > 0
        ? compact([try(site.vpn_gateway_ipv6, null), try(site.vpn_gateway_ipv4, null)])[0]
        : null
      ) : null
    })
  }

  vpn_manifest = {
    generated_at = timestamp()
    clouds       = var.cloud_manifests
    sites = {
      for name, site in local.site_details :
      name => {
        location             = site.location
        ipv4_cidr            = site.ipv4_cidr
        ipv6_prefix          = site.ipv6_prefix
        device_type          = site.device_type
        tunnel_count         = site.tunnel_count
        primary_asn          = site.primary_asn
        connect_to_cloud     = site.connect_to_cloud
        preferred_gateway_ip = site.preferred_gateway_ip
        vpn_gateway_ips = {
          ipv6 = site.vpn_gateway_ipv6
          ipv4 = site.vpn_gateway_ipv4
        }
        pre_shared_key = random_password.site_keys[name].result
      }
    }
  }
}

resource "random_password" "site_keys" {
  for_each = var.sites

  length  = 24
  special = false
}

resource "local_file" "vpn_manifest" {
  content  = jsonencode(local.vpn_manifest)
  filename = format("%s/%s", path.root, var.output_filename)
}
