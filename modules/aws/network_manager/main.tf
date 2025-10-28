locals {
  site_map    = { for site in var.sites : site.name => site }
  device_map  = { for device in var.devices : device.name => device }
  name_prefix = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
}

resource "aws_networkmanager_global_network" "this" {
  description = var.description

  tags = merge(var.tags, {
    Name = "${local.name_prefix}-${var.global_name}"
  })
}

resource "aws_networkmanager_site" "this" {
  for_each = local.site_map

  global_network_id = aws_networkmanager_global_network.this.id
  description       = try(each.value.description, null)

  dynamic "location" {
    for_each = compact([
      try(each.value.address, null) != null || try(each.value.latitude, null) != null || try(each.value.longitude, null) != null ? each.key : null
    ])
    content {
      address   = try(each.value.address, null)
      latitude  = try(each.value.latitude, null)
      longitude = try(each.value.longitude, null)
    }
  }

  tags = {
    for k, v in merge(var.tags, {
      Name           = "${local.name_prefix}-${each.value.name}"
      SkyforgeRegion = try(each.value.region, null)
    }) :
    k => v if v != null
  }
}

resource "aws_networkmanager_device" "this" {
  for_each = local.device_map

  global_network_id = aws_networkmanager_global_network.this.id
  site_id           = aws_networkmanager_site.this[each.value.site_name].id
  description       = try(each.value.description, null)
  model             = try(each.value.model, null)
  serial_number     = try(each.value.serial, null)
  type              = try(each.value.type, null)

  tags = {
    for k, v in merge(var.tags, {
      Name         = "${local.name_prefix}-${each.value.name}"
      SkyforgeSite = each.value.site_name
    }) :
    k => v if v != null
  }
}
