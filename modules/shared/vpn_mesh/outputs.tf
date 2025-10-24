output "cloud_links" {
  description = "Processed cloud-to-cloud VPN link definitions."
  value       = local.enriched_cloud_links
}

output "vnf_links" {
  description = "Processed VNF-to-cloud VPN link definitions with gateway metadata."
  value       = local.enriched_vnf_links
}
