output "global_network_id" {
  description = "Identifier of the created AWS Network Manager global network."
  value       = aws_networkmanager_global_network.this.id
}

output "site_ids" {
  description = "Map of site names to their identifiers."
  value = {
    for name, site in aws_networkmanager_site.this :
    name => site.id
  }
}
