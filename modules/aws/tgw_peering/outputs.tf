output "attachment_id" {
  description = "Transit Gateway peering attachment ID."
  value       = aws_ec2_transit_gateway_peering_attachment.this.id
}

output "accepter_attachment_id" {
  description = "Transit Gateway peering attachment accepter ID."
  value       = aws_ec2_transit_gateway_peering_attachment_accepter.this.id
}
