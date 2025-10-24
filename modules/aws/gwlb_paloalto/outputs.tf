output "gwlb_arn" {
  description = "ARN of the Gateway Load Balancer."
  value       = aws_lb.this.arn
}

output "target_group_arn" {
  description = "ARN of the Gateway Load Balancer target group."
  value       = aws_lb_target_group.this.arn
}

output "endpoint_service_id" {
  description = "Identifier of the Gateway Load Balancer endpoint service."
  value       = aws_vpc_endpoint_service.this.id
}

output "firewall_private_ips" {
  description = "Private IP addresses allocated to the Palo Alto firewall instances."
  value       = local.firewall_private_ips
}

output "firewall_instance_ids" {
  description = "Instance IDs for the Palo Alto firewall appliances."
  value       = local.firewall_instance_ids
}

output "admin_credentials" {
  description = "Administrative credentials for the Palo Alto firewalls (password may be null when not bootstrapped)."
  value = {
    username = local.firewall_admin_username
    password = local.firewall_admin_password
  }
  sensitive = true
}
