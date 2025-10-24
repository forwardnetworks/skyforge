output "transit_gateway_id" {
  description = "Transit Gateway ID for the AWS region (if created)."
  value       = try(aws_ec2_transit_gateway.this[0].id, null)
}

output "transit_gateway_route_table_id" {
  description = "Default Transit Gateway route table ID for the region."
  value       = try(aws_ec2_transit_gateway.this[0].association_default_route_table_id, null)
}

output "transit_gateway_connect" {
  description = "Transit Gateway Connect metadata when configured."
  value       = local.transit_gateway_connect_metadata
}

output "vpc_ids" {
  description = "Map of VPC IDs by logical name."
  value = {
    for name, vpc in aws_vpc.this :
    name => vpc.id
  }
}

output "subnets" {
  description = "Detailed subnet metadata for the region."
  value       = local.vpn_endpoints.spoke_vpcs
}

output "vpn_endpoint_metadata" {
  description = "Structured data describing outbound VPN endpoints for the AWS region."
  value       = local.vpn_endpoints
}

output "network_firewall" {
  description = "AWS Network Firewall metadata when deployed."
  value = local.network_firewall_enabled ? {
    firewall_arn       = aws_networkfirewall_firewall.this[0].arn
    firewall_policy    = aws_networkfirewall_firewall_policy.this[0].arn
    stateful_rule_arn  = aws_networkfirewall_rule_group.stateful[0].arn
    stateless_rule_arn = aws_networkfirewall_rule_group.stateless[0].arn
    subnet_ids         = local.network_firewall_subnet_ids
    log_group_name     = aws_cloudwatch_log_group.network_firewall[0].name
  } : null
}

output "fortinet_firewalls" {
  description = "Fortinet firewall instance metadata when enabled."
  value = local.fortinet_enabled ? {
    instance_ids = module.fortinet_firewalls[0].instance_ids
    private_ips  = module.fortinet_firewalls[0].private_ips
  } : null
}

output "checkpoint_firewalls" {
  description = "Check Point firewall instance metadata when enabled."
  value = local.checkpoint_enabled ? {
    instance_ids = module.checkpoint_firewalls[0].instance_ids
    private_ips  = module.checkpoint_firewalls[0].private_ips
  } : null
}

output "app_stack" {
  description = "Application workload stack metadata when enabled."
  value       = local.app_stack_ready ? module.app_stack[0].metadata : null
}

output "gateway_load_balancer" {
  description = "Metadata for the Palo Alto Gateway Load Balancer stack when enabled."
  value = length(module.paloalto_gwlb) > 0 ? {
    load_balancer_arn   = module.paloalto_gwlb[0].gwlb_arn
    target_group_arn    = module.paloalto_gwlb[0].target_group_arn
    endpoint_service_id = module.paloalto_gwlb[0].endpoint_service_id
    firewalls = {
      instance_ids      = module.paloalto_gwlb[0].firewall_instance_ids
      private_ips       = module.paloalto_gwlb[0].firewall_private_ips
      admin_credentials = module.paloalto_gwlb[0].admin_credentials
    }
  } : null
}

output "vpn_connections" {
  description = "AWS VPN connection metadata keyed by VNF site."
  value = {
    for site, conn in aws_vpn_connection.vnf :
    site => {
      id                 = conn.id
      customer_gateway   = aws_customer_gateway.vnf[site].id
      tunnel1_address    = conn.tunnel1_address
      tunnel1_cgw_inside = conn.tunnel1_cgw_inside_address
      tunnel1_vgw_inside = conn.tunnel1_vgw_inside_address
      tunnel2_address    = conn.tunnel2_address
      tunnel2_cgw_inside = conn.tunnel2_cgw_inside_address
      tunnel2_vgw_inside = conn.tunnel2_vgw_inside_address
      psk = {
        tunnel1 = random_password.vnf_tunnel1_psk[site].result
        tunnel2 = random_password.vnf_tunnel2_psk[site].result
      }
    }
  }
}

output "transit_gateway_attachments" {
  description = "Map of Transit Gateway VPC attachment IDs keyed by VPC name."
  value = var.region_config.enable_transit_gateway ? {
    for name, attachment in aws_ec2_transit_gateway_vpc_attachment.this :
    name => attachment.id
  } : {}
}
