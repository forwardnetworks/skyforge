locals {
  vnf_lookup = {
    for name, site in var.sites :
    name => site
  }

  enriched_vnf_links = [
    for link in var.mesh.vnf_links :
    merge(link, {
      site_details        = lookup(local.vnf_lookup, link.site, null)
      customer_gateway_ip = coalesce(link.customer_gateway_ipv6, link.customer_gateway_ipv4, try(local.vnf_lookup[link.site].preferred_gateway_ip, null))
    })
  ]

  enriched_cloud_links = [
    for link in var.mesh.cloud_links :
    merge(link, {
      link_id = format("%s-%s__%s-%s", link.source.cloud, link.source.region, link.target.cloud, link.target.region)
    })
  ]
}
