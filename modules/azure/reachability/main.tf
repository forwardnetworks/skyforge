locals {
  test_map = {
    for test in var.tests :
    test.name => test
  }
}

resource "azurerm_network_connection_monitor" "this" {
  for_each = local.test_map

  name               = "ncm-${var.region_key}-${replace(each.key, "[^A-Za-z0-9]", "")}"
  location           = var.location
  network_watcher_id = var.network_watcher_id
  notes              = try(each.value.description, null)

  tags = {
    for k, v in merge(var.tags, {
      Name           = "skyforge-${var.region_key}-${each.key}-connmon"
      SkyforgeRegion = var.region_key
    }) :
    k => v if v != null
  }

  endpoint {
    name    = "source"
    address = each.value.source_address
  }

  endpoint {
    name    = "destination"
    address = each.value.destination_address
  }

  test_configuration {
    name                      = "${each.key}-tcp"
    test_frequency_in_seconds = try(each.value.frequency_seconds, 60)
    protocol                  = title(lower(try(each.value.protocol, "tcp")))
    tcp_configuration {
      port                = tonumber(try(each.value.destination_port, 443))
      trace_route_enabled = try(each.value.trace_route_enabled, true)
    }
  }

  test_group {
    name                     = "${each.key}-group"
    source_endpoints         = ["source"]
    destination_endpoints    = ["destination"]
    test_configuration_names = ["${each.key}-tcp"]
    enabled                  = try(each.value.enabled, true)
  }
}
