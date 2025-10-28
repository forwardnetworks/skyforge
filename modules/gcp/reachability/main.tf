locals {
  project_id = var.project_id == null ? null : (trimspace(var.project_id) == "" ? null : trimspace(var.project_id))
  test_map = {
    for test in var.tests :
    test.name => test
    if try(test.enabled, true)
  }
  name_prefix = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
}

resource "google_network_management_connectivity_test" "this" {
  for_each = local.test_map

  name        = format("ct-%s-%s", local.name_prefix, replace(each.key, "[^A-Za-z0-9]", ""))
  project     = local.project_id
  protocol    = upper(try(each.value.protocol, "TCP"))
  description = try(each.value.description, null)

  source {
    ip_address = each.value.source_ip
    port       = try(tonumber(each.value.source_port), null)
    project_id = local.project_id
  }

  destination {
    ip_address = each.value.destination_ip
    port       = try(tonumber(each.value.destination_port), null)
    project_id = local.project_id
  }

  related_projects = try(each.value.related_projects, [])

  labels = {
    for k, v in merge(var.labels, {
      "skyforge_region" = var.region_key
    }) :
    replace(lower(k), "[^a-z0-9_]", "_") => substr(tostring(v), 0, 63)
    if v != null && replace(lower(k), "[^a-z0-9_]", "_") != ""
  }
}
