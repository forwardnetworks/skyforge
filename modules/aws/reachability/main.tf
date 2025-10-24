locals {
  attachment_lookup = var.transit_gateway_attachments
  path_map          = { for path in var.paths : path.name => path }
}

resource "aws_ec2_network_insights_path" "this" {
  for_each = local.path_map

  source           = lookup(local.attachment_lookup, each.value.source_vpc, null)
  destination      = lookup(local.attachment_lookup, each.value.destination_vpc, null)
  protocol         = lower(try(each.value.protocol, "tcp"))
  destination_port = try(each.value.destination_port, null)
  tags = {
    for k, v in merge(var.tags, {
      Name           = "skyforge-${var.region_key}-${each.key}-reachability"
      SkyforgeRegion = var.region_key
    }) :
    k => v if v != null
  }

  lifecycle {
    precondition {
      condition     = lookup(local.attachment_lookup, each.value.source_vpc, null) != null
      error_message = "Transit Gateway attachment for source VPC '${each.value.source_vpc}' was not found."
    }
    precondition {
      condition     = lookup(local.attachment_lookup, each.value.destination_vpc, null) != null
      error_message = "Transit Gateway attachment for destination VPC '${each.value.destination_vpc}' was not found."
    }
  }
}

resource "aws_ec2_network_insights_analysis" "this" {
  for_each = {
    for name, path in aws_ec2_network_insights_path.this :
    name => path if try(local.path_map[name].perform_analysis, true)
  }

  network_insights_path_id = each.value.id
  tags = {
    for k, v in merge(var.tags, {
      Name           = "skyforge-${var.region_key}-${each.key}-analysis"
      SkyforgeRegion = var.region_key
    }) :
    k => v if v != null
  }
}
