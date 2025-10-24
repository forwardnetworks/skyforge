locals {
  record_entries = {
    for key, rec in var.records :
    key => rec
  }

  alias_records = {
    for key, rec in local.record_entries :
    key => rec if try(rec.alias, null) != null
  }

  standard_records = {
    for key, rec in local.record_entries :
    key => rec if try(rec.alias, null) == null
  }
}

resource "aws_route53_zone" "this" {
  name          = var.domain_name
  comment       = var.comment
  force_destroy = true
  tags          = var.tags

  dynamic "vpc" {
    for_each = var.private_zone ? var.vpc_associations : []
    content {
      vpc_id     = vpc.value.vpc_id
      vpc_region = vpc.value.region
    }
  }

  lifecycle {
    precondition {
      condition     = (!var.private_zone) || length(var.vpc_associations) > 0
      error_message = "Private hosted zones require at least one VPC association."
    }
  }
}

resource "aws_route53_record" "standard" {
  for_each = local.standard_records

  zone_id = aws_route53_zone.this.zone_id
  name    = coalesce(try(each.value.name, null), each.key)
  type    = each.value.type
  ttl     = try(each.value.ttl, 300)
  records = try(each.value.values, [])

  depends_on = [aws_route53_zone.this]
}

resource "aws_route53_record" "alias" {
  for_each = local.alias_records

  zone_id = aws_route53_zone.this.zone_id
  name    = coalesce(try(each.value.name, null), each.key)
  type    = each.value.type

  alias {
    name                   = each.value.alias.name
    zone_id                = each.value.alias.zone_id
    evaluate_target_health = try(each.value.alias.evaluate_target_health, false)
  }

  depends_on = [aws_route53_zone.this]
}
