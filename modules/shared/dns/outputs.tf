output "zone" {
  description = "Route53 zone metadata."
  value = {
    zone_id      = aws_route53_zone.this.zone_id
    name         = aws_route53_zone.this.name
    name_servers = aws_route53_zone.this.name_servers
  }
}
