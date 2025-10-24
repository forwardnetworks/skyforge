output "vpn_manifest" {
  description = "Structured manifest of VNF VPN endpoints and generated secrets."
  value       = local.vpn_manifest
}

output "pre_shared_keys" {
  description = "Randomly generated pre-shared keys per VNF endpoint."
  value = {
    for name, password in random_password.site_keys :
    name => password.result
  }
  sensitive = true
}
