locals {
  name_prefix = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
  link_name   = "${local.name_prefix}-tgw-peering-${var.requester_region}-to-${var.peer_region}"
}

resource "aws_ec2_transit_gateway_peering_attachment" "this" {
  provider = aws.requester

  transit_gateway_id      = var.requester_tgw_id
  peer_transit_gateway_id = var.peer_tgw_id
  peer_region             = var.peer_region

  tags = merge(var.tags, {
    Name          = local.link_name
    SkyforgeRole  = "tgw-mesh"
    SkyforgeLink  = local.link_name
    SkyforgeScope = "requester"
  })
}

resource "aws_ec2_transit_gateway_peering_attachment_accepter" "this" {
  provider = aws.peer

  transit_gateway_attachment_id = aws_ec2_transit_gateway_peering_attachment.this.id

  tags = merge(var.tags, {
    Name          = local.link_name
    SkyforgeRole  = "tgw-mesh"
    SkyforgeLink  = local.link_name
    SkyforgeScope = "peer"
  })
}

resource "aws_ec2_transit_gateway_route" "requester" {
  provider = aws.requester

  for_each = { for cidr in var.requester_destination_cidrs : cidr => cidr }

  destination_cidr_block         = each.value
  transit_gateway_attachment_id  = aws_ec2_transit_gateway_peering_attachment.this.id
  transit_gateway_route_table_id = var.requester_route_table_id
}

resource "aws_ec2_transit_gateway_route" "peer" {
  provider = aws.peer

  for_each = { for cidr in var.peer_destination_cidrs : cidr => cidr }

  destination_cidr_block         = each.value
  transit_gateway_attachment_id  = aws_ec2_transit_gateway_peering_attachment_accepter.this.id
  transit_gateway_route_table_id = var.peer_route_table_id
}
