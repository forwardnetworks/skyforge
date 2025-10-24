locals {
  aws_region_configs = {
    "us-east-1"      = lookup(var.aws_regions, "us-east-1", null)
    "eu-central-1"   = lookup(var.aws_regions, "eu-central-1", null)
    "ap-northeast-1" = lookup(var.aws_regions, "ap-northeast-1", null)
    "me-south-1"     = lookup(var.aws_regions, "me-south-1", null)
  }

  gcp_region_configs = {
    "us-central1"     = lookup(var.gcp_regions, "us-central1", null)
    "europe-west1"    = lookup(var.gcp_regions, "europe-west1", null)
    "asia-southeast1" = lookup(var.gcp_regions, "asia-southeast1", null)
    "me-west1"        = lookup(var.gcp_regions, "me-west1", null)
  }

  azure_resource_group_settings = var.azure_global_resource_group != null ? merge({
    create_if_missing = false
    tags              = {}
    }, var.azure_global_resource_group) : (
    var.azure_resource_group_name != null ? {
      name              = var.azure_resource_group_name
      location          = null
      create_if_missing = false
      tags              = {}
    } : null
  )
}

locals {
  aws_vnf_links_by_region = {
    for region, config in local.aws_region_configs :
    region => [
      for link in var.vpn_mesh.vnf_links :
      link if link.cloud == "aws" && link.region == region
    ] if config != null
  }

  azure_vnf_links_by_region = {
    for region, config in var.azure_regions :
    region => [
      for link in var.vpn_mesh.vnf_links :
      link if link.cloud == "azure" && link.region == region
    ]
  }

  gcp_vnf_links_by_region = {
    for region, config in local.gcp_region_configs :
    region => [
      for link in var.vpn_mesh.vnf_links :
      link if link.cloud == "gcp" && link.region == region
    ] if config != null
  }
}

module "aws_us_east_1" {
  count         = local.aws_region_configs["us-east-1"] == null ? 0 : 1
  source        = "./modules/aws"
  region_key    = "us-east-1"
  region_config = local.aws_region_configs["us-east-1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.aws_vnf_links_by_region, "us-east-1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    aws = aws.us-east-1
  }
}

module "aws_eu_central_1" {
  count         = local.aws_region_configs["eu-central-1"] == null ? 0 : 1
  source        = "./modules/aws"
  region_key    = "eu-central-1"
  region_config = local.aws_region_configs["eu-central-1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.aws_vnf_links_by_region, "eu-central-1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    aws = aws.eu-central-1
  }
}

module "aws_ap_northeast_1" {
  count         = local.aws_region_configs["ap-northeast-1"] == null ? 0 : 1
  source        = "./modules/aws"
  region_key    = "ap-northeast-1"
  region_config = local.aws_region_configs["ap-northeast-1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.aws_vnf_links_by_region, "ap-northeast-1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    aws = aws.ap-northeast-1
  }
}

module "aws_me_south_1" {
  count         = local.aws_region_configs["me-south-1"] == null ? 0 : 1
  source        = "./modules/aws"
  region_key    = "me-south-1"
  region_config = local.aws_region_configs["me-south-1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.aws_vnf_links_by_region, "me-south-1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    aws = aws.me-south-1
  }
}

module "aws_network_manager" {
  count = local.aws_network_manager_enabled ? 1 : 0

  source = "./modules/aws/network_manager"

  global_name = try(var.aws_network_manager.global_name, "skyforge-global-network")
  description = try(var.aws_network_manager.description, null)
  sites       = local.aws_network_manager_sites
  devices     = local.aws_network_manager_devices
  tags        = var.default_tags

  providers = {
    aws = aws.us-east-1
  }
}

module "aws_reachability_us_east_1" {
  count = length(try(var.aws_reachability["us-east-1"].paths, [])) > 0 ? 1 : 0

  source = "./modules/aws/reachability"

  region_key                  = "us-east-1"
  paths                       = try(var.aws_reachability["us-east-1"].paths, [])
  transit_gateway_attachments = lookup(local.aws_transit_gateway_attachment_map, "us-east-1", {})
  tags                        = var.default_tags

  providers = {
    aws = aws.us-east-1
  }
}

module "aws_reachability_eu_central_1" {
  count = length(try(var.aws_reachability["eu-central-1"].paths, [])) > 0 ? 1 : 0

  source = "./modules/aws/reachability"

  region_key                  = "eu-central-1"
  paths                       = try(var.aws_reachability["eu-central-1"].paths, [])
  transit_gateway_attachments = lookup(local.aws_transit_gateway_attachment_map, "eu-central-1", {})
  tags                        = var.default_tags

  providers = {
    aws = aws.eu-central-1
  }
}

module "aws_reachability_ap_northeast_1" {
  count = length(try(var.aws_reachability["ap-northeast-1"].paths, [])) > 0 ? 1 : 0

  source = "./modules/aws/reachability"

  region_key                  = "ap-northeast-1"
  paths                       = try(var.aws_reachability["ap-northeast-1"].paths, [])
  transit_gateway_attachments = lookup(local.aws_transit_gateway_attachment_map, "ap-northeast-1", {})
  tags                        = var.default_tags

  providers = {
    aws = aws.ap-northeast-1
  }
}

module "aws_reachability_me_south_1" {
  count = length(try(var.aws_reachability["me-south-1"].paths, [])) > 0 ? 1 : 0

  source = "./modules/aws/reachability"

  region_key                  = "me-south-1"
  paths                       = try(var.aws_reachability["me-south-1"].paths, [])
  transit_gateway_attachments = lookup(local.aws_transit_gateway_attachment_map, "me-south-1", {})
  tags                        = var.default_tags

  providers = {
    aws = aws.me-south-1
  }
}

module "azure_resource_group" {
  count = local.azure_resource_group_settings != null ? 1 : 0

  source = "./modules/azure/resource_group"

  name              = local.azure_resource_group_settings.name
  location          = try(local.azure_resource_group_settings.location, null)
  create_if_missing = try(local.azure_resource_group_settings.create_if_missing, false)
  tags              = merge(var.default_tags, try(local.azure_resource_group_settings.tags, {}))
}

locals {
  azure_resource_group_name_effective = local.azure_resource_group_settings != null ? (
    length(module.azure_resource_group) > 0 ? module.azure_resource_group[0].name : local.azure_resource_group_settings.name
  ) : null
  azure_resource_group_location_effective = local.azure_resource_group_settings != null ? (
    length(module.azure_resource_group) > 0 ? module.azure_resource_group[0].location : try(local.azure_resource_group_settings.location, null)
  ) : null
}

module "azure_regions" {
  for_each            = var.azure_regions
  source              = "./modules/azure"
  region_key          = each.key
  region_config       = each.value
  default_tags        = var.default_tags
  vnf_links           = lookup(local.azure_vnf_links_by_region, each.key, [])
  vnf_sites           = var.vnf_endpoints
  resource_group_name = local.azure_resource_group_name_effective

  providers = {
    azurerm = azurerm
  }
}

module "azure_reachability" {
  for_each = {
    for region, config in var.azure_reachability :
    region => config
    if contains(keys(module.azure_regions), region) && try(local.azure_network_watcher_map[region], null) != null && length(try(config.tests, [])) > 0
  }

  source = "./modules/azure/reachability"

  region_key         = each.key
  location           = module.azure_regions[each.key].location
  network_watcher_id = local.azure_network_watcher_map[each.key]
  tests              = coalesce(each.value.tests, [])
  tags               = var.default_tags
}


module "gcp_us_central1" {
  count         = local.gcp_region_configs["us-central1"] == null ? 0 : 1
  source        = "./modules/gcp"
  region_key    = "us-central1"
  region_config = local.gcp_region_configs["us-central1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.gcp_vnf_links_by_region, "us-central1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    google      = google.us_central1
    google-beta = google-beta.us_central1
  }
}

module "gcp_europe_west1" {
  count         = local.gcp_region_configs["europe-west1"] == null ? 0 : 1
  source        = "./modules/gcp"
  region_key    = "europe-west1"
  region_config = local.gcp_region_configs["europe-west1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.gcp_vnf_links_by_region, "europe-west1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    google      = google.europe_west1
    google-beta = google-beta.europe_west1
  }
}

module "gcp_asia_southeast1" {
  count         = local.gcp_region_configs["asia-southeast1"] == null ? 0 : 1
  source        = "./modules/gcp"
  region_key    = "asia-southeast1"
  region_config = local.gcp_region_configs["asia-southeast1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.gcp_vnf_links_by_region, "asia-southeast1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    google      = google.asia_southeast1
    google-beta = google-beta.asia_southeast1
  }
}

module "gcp_me_west1" {
  count         = local.gcp_region_configs["me-west1"] == null ? 0 : 1
  source        = "./modules/gcp"
  region_key    = "me-west1"
  region_config = local.gcp_region_configs["me-west1"]
  default_tags  = var.default_tags
  vnf_links     = lookup(local.gcp_vnf_links_by_region, "me-west1", [])
  vnf_sites     = var.vnf_endpoints

  providers = {
    google      = google.me_west1
    google-beta = google-beta.me_west1
  }
}

module "gcp_reachability" {
  for_each = {
    for region, config in var.gcp_reachability :
    region => config
    if local.gcp_region_configs[region] != null && length(try(config.tests, [])) > 0
  }

  source = "./modules/gcp/reachability"

  region_key = each.key
  project_id = coalesce(try(each.value.project_id, null), try(local.gcp_project_map[each.key], null))
  tests      = coalesce(each.value.tests, [])
  labels     = var.default_tags
}


module "dns" {
  count = try(var.dns.enable, false) ? 1 : 0

  source = "./modules/shared/dns"

  domain_name      = try(var.dns.domain, null)
  comment          = try(var.dns.comment, "Skyforge DNS zone")
  private_zone     = try(var.dns.private_zone, false)
  vpc_associations = try(var.dns.vpc_associations, [])
  tags             = merge(var.default_tags, try(var.dns.tags, {}))
  records          = try(var.dns.records, {})
}

module "vpn_mesh" {
  source = "./modules/shared/vpn_mesh"
  mesh   = var.vpn_mesh
  sites  = var.vnf_endpoints
}

locals {
  aws_vpn_manifest_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].vpn_endpoint_metadata } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].vpn_endpoint_metadata } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].vpn_endpoint_metadata } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].vpn_endpoint_metadata } : {}
  )

  aws_app_stack_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].app_stack } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].app_stack } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].app_stack } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].app_stack } : {}
  )
  aws_transit_gateway_connect_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].transit_gateway_connect } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].transit_gateway_connect } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].transit_gateway_connect } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].transit_gateway_connect } : {}
  )
  aws_global_accelerator_map = {
    for region, metadata in local.aws_app_stack_map :
    region => (metadata != null ? try(metadata.global_accelerator, null) : null)
  }
  aws_app_alb_metadata = {
    for region, metadata in local.aws_app_stack_map :
    region => try(metadata.alb, null)
  }
  aws_global_alb_endpoints = {
    for region, config in local.aws_region_configs :
    region => {
      load_balancer_arn = try(local.aws_app_alb_metadata[region].load_balancer_arn, null)
      dns_name          = try(local.aws_app_alb_metadata[region].dns_name, null)
      zone_id           = try(local.aws_app_alb_metadata[region].zone_id, null)
    }
    if config != null && try(config.app_stack.enable, false) && try(config.app_stack.create_alb, false)
  }
  aws_global_app_config = merge({
    enable           = true
    listener_ports   = [80, 443]
    health_check     = {}
    endpoint_weights = {}
    record_name      = "global-app"
  }, var.aws_global_app)
  aws_global_app_listener_ports   = length(coalesce(local.aws_global_app_config.listener_ports, [])) > 0 ? distinct(coalesce(local.aws_global_app_config.listener_ports, [])) : [80, 443]
  aws_global_app_endpoint_weights = coalesce(local.aws_global_app_config.endpoint_weights, {})
  aws_global_app_health_check     = coalesce(local.aws_global_app_config.health_check, {})
  aws_global_app_enabled          = try(local.aws_global_app_config.enable, true)
  aws_global_app_record_name      = trimspace(try(local.aws_global_app_config.record_name, ""))
  aws_gwlb_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].gateway_load_balancer } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].gateway_load_balancer } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].gateway_load_balancer } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].gateway_load_balancer } : {}
  )

  aws_vpn_connection_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].vpn_connections } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].vpn_connections } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].vpn_connections } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].vpn_connections } : {}
  )

  gcp_vpn_manifest_map = merge(
    local.gcp_region_configs["us-central1"] != null ? { "us-central1" = module.gcp_us_central1[0].vpn_manifest } : {},
    local.gcp_region_configs["europe-west1"] != null ? { "europe-west1" = module.gcp_europe_west1[0].vpn_manifest } : {},
    local.gcp_region_configs["asia-southeast1"] != null ? { "asia-southeast1" = module.gcp_asia_southeast1[0].vpn_manifest } : {},
    local.gcp_region_configs["me-west1"] != null ? { "me-west1" = module.gcp_me_west1[0].vpn_manifest } : {}
  )

  gcp_vpn_connection_map = merge(
    local.gcp_region_configs["us-central1"] != null ? { "us-central1" = module.gcp_us_central1[0].vpn_connections } : {},
    local.gcp_region_configs["europe-west1"] != null ? { "europe-west1" = module.gcp_europe_west1[0].vpn_connections } : {},
    local.gcp_region_configs["asia-southeast1"] != null ? { "asia-southeast1" = module.gcp_asia_southeast1[0].vpn_connections } : {},
    local.gcp_region_configs["me-west1"] != null ? { "me-west1" = module.gcp_me_west1[0].vpn_connections } : {}
  )

  gcp_workloads_map = merge(
    local.gcp_region_configs["us-central1"] != null ? { "us-central1" = module.gcp_us_central1[0].workloads } : {},
    local.gcp_region_configs["europe-west1"] != null ? { "europe-west1" = module.gcp_europe_west1[0].workloads } : {},
    local.gcp_region_configs["asia-southeast1"] != null ? { "asia-southeast1" = module.gcp_asia_southeast1[0].workloads } : {},
    local.gcp_region_configs["me-west1"] != null ? { "me-west1" = module.gcp_me_west1[0].workloads } : {}
  )
  azure_workloads_map = {
    for region, mod in module.azure_regions :
    region => mod.workloads
  }
  azure_front_door_map = {
    for region, workloads in local.azure_workloads_map :
    region => (workloads != null ? try(workloads.front_door, null) : null)
  }
  azure_asa_map = {
    for region, mod in module.azure_regions :
    region => mod.asa_nva
  }
  azure_network_watcher_map = {
    for region, mod in module.azure_regions :
    region => mod.network_watcher_id
  }
  gcp_global_lb_map = {
    for region, workloads in local.gcp_workloads_map :
    region => (workloads != null ? try(workloads.global_load_balancer, null) : null)
  }
  gcp_checkpoint_map = merge(
    local.gcp_region_configs["us-central1"] != null ? { "us-central1" = module.gcp_us_central1[0].checkpoint_firewall } : {},
    local.gcp_region_configs["europe-west1"] != null ? { "europe-west1" = module.gcp_europe_west1[0].checkpoint_firewall } : {},
    local.gcp_region_configs["asia-southeast1"] != null ? { "asia-southeast1" = module.gcp_asia_southeast1[0].checkpoint_firewall } : {},
    local.gcp_region_configs["me-west1"] != null ? { "me-west1" = module.gcp_me_west1[0].checkpoint_firewall } : {}
  )
  azure_shared_vnet_ids = {
    for region, mod in module.azure_regions :
    region => try(mod.virtual_network_ids["shared-services"], null)
  }
  azure_mesh_regions = sort([
    for region, config in var.azure_regions :
    region if config != null
  ])
  azure_vnet_mesh_pairs = flatten([
    for idx, source in local.azure_mesh_regions : [
      for target in slice(local.azure_mesh_regions, idx + 1, length(local.azure_mesh_regions)) : {
        source_region = source
        target_region = target
      }
    ]
  ])
  azure_vnet_mesh_entries = flatten([
    for pair in local.azure_vnet_mesh_pairs : [
      {
        key           = "${pair.source_region}__${pair.target_region}"
        source_region = pair.source_region
        target_region = pair.target_region
      },
      {
        key           = "${pair.target_region}__${pair.source_region}"
        source_region = pair.target_region
        target_region = pair.source_region
      }
    ]
  ])
  azure_vnet_peering_map = {
    for entry in local.azure_vnet_mesh_entries :
    entry.key => {
      source_region = entry.source_region
      target_region = entry.target_region
    }
  }
  gcp_shared_services_networks = merge(
    local.gcp_region_configs["us-central1"] != null ? { "us-central1" = try(module.gcp_us_central1[0].network_ids["shared-services"], null) } : {},
    local.gcp_region_configs["europe-west1"] != null ? { "europe-west1" = try(module.gcp_europe_west1[0].network_ids["shared-services"], null) } : {},
    local.gcp_region_configs["asia-southeast1"] != null ? { "asia-southeast1" = try(module.gcp_asia_southeast1[0].network_ids["shared-services"], null) } : {},
    local.gcp_region_configs["me-west1"] != null ? { "me-west1" = try(module.gcp_me_west1[0].network_ids["shared-services"], null) } : {}
  )
  gcp_mesh_regions = sort([
    for region, config in local.gcp_region_configs :
    region if config != null
  ])
  gcp_network_mesh_pairs = flatten([
    for idx, source in local.gcp_mesh_regions : [
      for target in slice(local.gcp_mesh_regions, idx + 1, length(local.gcp_mesh_regions)) : {
        source_region = source
        target_region = target
      }
    ]
  ])
  gcp_network_mesh_entries = flatten([
    for pair in local.gcp_network_mesh_pairs : [
      {
        key           = "${pair.source_region}__${pair.target_region}"
        source_region = pair.source_region
        target_region = pair.target_region
      },
      {
        key           = "${pair.target_region}__${pair.source_region}"
        source_region = pair.target_region
        target_region = pair.source_region
      }
    ]
  ])
  gcp_network_mesh_map = {
    for entry in local.gcp_network_mesh_entries :
    entry.key => {
      source_region = entry.source_region
      target_region = entry.target_region
    }
  }
  gcp_project_map = {
    for region, config in local.gcp_region_configs :
    region => try(config.project_id, null)
  }
  aws_reachability_paths = merge(
    length(module.aws_reachability_us_east_1) > 0 ? { "us-east-1" = module.aws_reachability_us_east_1[0].path_ids } : {},
    length(module.aws_reachability_eu_central_1) > 0 ? { "eu-central-1" = module.aws_reachability_eu_central_1[0].path_ids } : {},
    length(module.aws_reachability_ap_northeast_1) > 0 ? { "ap-northeast-1" = module.aws_reachability_ap_northeast_1[0].path_ids } : {},
    length(module.aws_reachability_me_south_1) > 0 ? { "me-south-1" = module.aws_reachability_me_south_1[0].path_ids } : {}
  )
  aws_reachability_analyses = merge(
    length(module.aws_reachability_us_east_1) > 0 ? { "us-east-1" = module.aws_reachability_us_east_1[0].analysis_ids } : {},
    length(module.aws_reachability_eu_central_1) > 0 ? { "eu-central-1" = module.aws_reachability_eu_central_1[0].analysis_ids } : {},
    length(module.aws_reachability_ap_northeast_1) > 0 ? { "ap-northeast-1" = module.aws_reachability_ap_northeast_1[0].analysis_ids } : {},
    length(module.aws_reachability_me_south_1) > 0 ? { "me-south-1" = module.aws_reachability_me_south_1[0].analysis_ids } : {}
  )
  azure_reachability_map = {
    for region, mod in module.azure_reachability :
    region => mod.connection_monitors
  }
  gcp_reachability_map = {
    for region, mod in module.gcp_reachability :
    region => mod.connectivity_tests
  }

  cloud_vpn_manifests_raw = {
    aws = {
      for region, manifest in local.aws_vpn_manifest_map :
      region => merge(jsondecode(manifest == null ? "{}" : jsonencode(manifest)), {
        connections       = lookup(local.aws_vpn_connection_map, region, {})
        application_stack = lookup(local.aws_app_stack_map, region, null)
      })
    }
    azure = {
      for region, mod in module.azure_regions :
      region => merge(jsondecode(mod.vpn_manifest == null ? "{}" : jsonencode(mod.vpn_manifest)), {
        connections    = jsondecode(mod.vpn_connections == null ? "{}" : jsonencode(mod.vpn_connections))
        vpn_gateway_id = mod.vpn_gateway_id
      })
    }
    gcp = {
      for region, manifest in local.gcp_vpn_manifest_map :
      region => merge(jsondecode(manifest == null ? "{}" : jsonencode(manifest)), {
        connections = lookup(local.gcp_vpn_connection_map, region, {}),
        workloads   = lookup(local.gcp_workloads_map, region, null)
      })
    }
    mesh = {
      cloud_links = module.vpn_mesh.cloud_links
      vnf_links   = module.vpn_mesh.vnf_links
      link_status = {
        for link_id, link in local.aws_cloud_links :
        link_id => {
          source = link.source
          target = link.target
          aws = contains(keys(local.aws_cloud_link_connections), link_id) ? {
            vpn_connection_id   = local.aws_cloud_link_connections[link_id].id
            customer_gateway_id = local.aws_cloud_link_customer_gateways[link_id].id
            tunnel1 = {
              outside_address    = local.aws_cloud_link_connections[link_id].tunnel1_address
              inside_cidr        = local.aws_cloud_link_connections[link_id].tunnel1_inside_cidr
              vgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel1_vgw_inside_address
              cgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel1_cgw_inside_address
              preshared_key      = random_password.cloud_link_tunnel1_psk[link_id].result
            }
            tunnel2 = {
              outside_address    = local.aws_cloud_link_connections[link_id].tunnel2_address
              inside_cidr        = local.aws_cloud_link_connections[link_id].tunnel2_inside_cidr
              vgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel2_vgw_inside_address
              cgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel2_cgw_inside_address
              preshared_key      = random_password.cloud_link_tunnel2_psk[link_id].result
            }
          } : null
          azure = contains(keys(local.azure_cloud_link_connections), link_id) ? {
            vpn_connection_id = local.azure_cloud_link_connections[link_id].id
            vpn_site_id       = local.azure_cloud_link_sites[link_id].id
            location          = module.azure_regions[link.target.region].location
          } : null
          gcp = contains(keys(local.gcp_cloud_link_vpn_tunnels), link_id) ? {
            vpn_tunnel_id    = local.gcp_cloud_link_vpn_tunnels[link_id].id
            router_interface = local.gcp_cloud_link_router_interfaces[link_id].name
            router_peer      = local.gcp_cloud_link_router_peers[link_id].name
            external_gateway = local.gcp_cloud_link_external_gateways[link_id].id
          } : null
        }
        if contains(keys(local.aws_cloud_link_connections), link_id)
      }
    }
  }
  cloud_vpn_manifests = jsondecode(jsonencode(local.cloud_vpn_manifests_raw))

  aws_transit_gateway_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].transit_gateway_id } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].transit_gateway_id } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].transit_gateway_id } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].transit_gateway_id } : {}
  )
  aws_transit_gateway_attachment_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].transit_gateway_attachments } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].transit_gateway_attachments } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].transit_gateway_attachments } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].transit_gateway_attachments } : {}
  )
  aws_network_manager_enabled = try(var.aws_network_manager.enable, false)
  aws_network_manager_sites = local.aws_network_manager_enabled ? (
    length(try(var.aws_network_manager.sites, [])) > 0 ?
    var.aws_network_manager.sites :
    [
      for region, cfg in local.aws_region_configs : {
        name        = "skyforge-${region}-site"
        description = "Skyforge ${region} site"
        region      = region
      } if cfg != null
    ]
  ) : []
  aws_network_manager_devices = local.aws_network_manager_enabled ? (
    length(try(var.aws_network_manager.devices, [])) > 0 ?
    var.aws_network_manager.devices :
    [
      for region, cfg in local.aws_region_configs : {
        name        = "skyforge-${region}-tgw"
        site_name   = "skyforge-${region}-site"
        type        = "transit-gateway"
        description = "Transit Gateway anchor for ${region}"
      } if cfg != null && try(cfg.enable_transit_gateway, false)
    ]
  ) : []

  aws_transit_gateway_route_table_map = merge(
    local.aws_region_configs["us-east-1"] != null ? { "us-east-1" = module.aws_us_east_1[0].transit_gateway_route_table_id } : {},
    local.aws_region_configs["eu-central-1"] != null ? { "eu-central-1" = module.aws_eu_central_1[0].transit_gateway_route_table_id } : {},
    local.aws_region_configs["ap-northeast-1"] != null ? { "ap-northeast-1" = module.aws_ap_northeast_1[0].transit_gateway_route_table_id } : {},
    local.aws_region_configs["me-south-1"] != null ? { "me-south-1" = module.aws_me_south_1[0].transit_gateway_route_table_id } : {}
  )

  aws_tgw_enabled_regions = [
    for region, config in local.aws_region_configs :
    region if config != null && try(config.enable_transit_gateway, false)
  ]

  aws_tgw_mesh_links_input_normalized = [
    for link in var.aws_tgw_mesh_links :
    {
      source_region = sort([link.source_region, link.target_region])[0]
      target_region = sort([link.source_region, link.target_region])[1]
    }
    if link.source_region != link.target_region
  ]

  aws_default_tgw_mesh_links = flatten([
    for idx, source_region in local.aws_tgw_enabled_regions : [
      for target_region in slice(local.aws_tgw_enabled_regions, idx + 1, length(local.aws_tgw_enabled_regions)) : {
        source_region = source_region
        target_region = target_region
      }
    ]
  ])

  aws_tgw_mesh_links_raw = length(local.aws_tgw_mesh_links_input_normalized) > 0 ? distinct(local.aws_tgw_mesh_links_input_normalized) : local.aws_default_tgw_mesh_links

  aws_tgw_mesh_links = [
    for link in local.aws_tgw_mesh_links_raw :
    {
      key           = "${link.source_region}__${link.target_region}"
      source_region = link.source_region
      target_region = link.target_region
    }
    if contains(local.aws_tgw_enabled_regions, link.source_region) && contains(local.aws_tgw_enabled_regions, link.target_region)
  ]

  aws_tgw_mesh_links_map = {
    for link in local.aws_tgw_mesh_links :
    link.key => link
  }

  aws_region_cidr_blocks = {
    for region, config in local.aws_region_configs :
    region => try(config.cidr_block, null)
  }

  aws_tgw_mesh_link_us_east_1__eu_central_1      = lookup(local.aws_tgw_mesh_links_map, "us-east-1__eu-central-1", null)
  aws_tgw_mesh_link_us_east_1__ap_northeast_1    = lookup(local.aws_tgw_mesh_links_map, "us-east-1__ap-northeast-1", null)
  aws_tgw_mesh_link_us_east_1__me_south_1        = lookup(local.aws_tgw_mesh_links_map, "us-east-1__me-south-1", null)
  aws_tgw_mesh_link_eu_central_1__ap_northeast_1 = lookup(local.aws_tgw_mesh_links_map, "eu-central-1__ap-northeast-1", null)
  aws_tgw_mesh_link_eu_central_1__me_south_1     = lookup(local.aws_tgw_mesh_links_map, "eu-central-1__me-south-1", null)
  aws_tgw_mesh_link_ap_northeast_1__me_south_1   = lookup(local.aws_tgw_mesh_links_map, "ap-northeast-1__me-south-1", null)

  gcp_ha_vpn_map = merge(
    local.gcp_region_configs["us-central1"] != null ? { "us-central1" = module.gcp_us_central1[0].ha_vpn_gateway_self_link } : {},
    local.gcp_region_configs["europe-west1"] != null ? { "europe-west1" = module.gcp_europe_west1[0].ha_vpn_gateway_self_link } : {},
    local.gcp_region_configs["asia-southeast1"] != null ? { "asia-southeast1" = module.gcp_asia_southeast1[0].ha_vpn_gateway_self_link } : {},
    local.gcp_region_configs["me-west1"] != null ? { "me-west1" = module.gcp_me_west1[0].ha_vpn_gateway_self_link } : {}
  )
}

module "vnfs" {
  source          = "./modules/shared/vnfs"
  sites           = var.vnf_endpoints
  cloud_manifests = local.cloud_vpn_manifests
}

module "aws_tgw_mesh_us_east_1__eu_central_1" {
  count = local.aws_tgw_mesh_link_us_east_1__eu_central_1 == null ? 0 : 1

  source = "./modules/aws/tgw_peering"

  requester_region            = local.aws_tgw_mesh_link_us_east_1__eu_central_1.source_region
  requester_tgw_id            = local.aws_transit_gateway_map["us-east-1"]
  requester_route_table_id    = local.aws_transit_gateway_route_table_map["us-east-1"]
  requester_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "eu-central-1", null)])

  peer_region            = local.aws_tgw_mesh_link_us_east_1__eu_central_1.target_region
  peer_tgw_id            = local.aws_transit_gateway_map["eu-central-1"]
  peer_route_table_id    = local.aws_transit_gateway_route_table_map["eu-central-1"]
  peer_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "us-east-1", null)])

  tags = merge(var.default_tags, {
    SkyforgeRole = "tgw-mesh"
  })

  providers = {
    aws.requester = aws.us-east-1
    aws.peer      = aws.eu-central-1
  }
}

module "aws_tgw_mesh_us_east_1__ap_northeast_1" {
  count = local.aws_tgw_mesh_link_us_east_1__ap_northeast_1 == null ? 0 : 1

  source = "./modules/aws/tgw_peering"

  requester_region            = local.aws_tgw_mesh_link_us_east_1__ap_northeast_1.source_region
  requester_tgw_id            = local.aws_transit_gateway_map["us-east-1"]
  requester_route_table_id    = local.aws_transit_gateway_route_table_map["us-east-1"]
  requester_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "ap-northeast-1", null)])

  peer_region            = local.aws_tgw_mesh_link_us_east_1__ap_northeast_1.target_region
  peer_tgw_id            = local.aws_transit_gateway_map["ap-northeast-1"]
  peer_route_table_id    = local.aws_transit_gateway_route_table_map["ap-northeast-1"]
  peer_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "us-east-1", null)])

  tags = merge(var.default_tags, {
    SkyforgeRole = "tgw-mesh"
  })

  providers = {
    aws.requester = aws.us-east-1
    aws.peer      = aws.ap-northeast-1
  }
}

module "aws_tgw_mesh_us_east_1__me_south_1" {
  count = local.aws_tgw_mesh_link_us_east_1__me_south_1 == null ? 0 : 1

  source = "./modules/aws/tgw_peering"

  requester_region            = local.aws_tgw_mesh_link_us_east_1__me_south_1.source_region
  requester_tgw_id            = local.aws_transit_gateway_map["us-east-1"]
  requester_route_table_id    = local.aws_transit_gateway_route_table_map["us-east-1"]
  requester_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "me-south-1", null)])

  peer_region            = local.aws_tgw_mesh_link_us_east_1__me_south_1.target_region
  peer_tgw_id            = local.aws_transit_gateway_map["me-south-1"]
  peer_route_table_id    = local.aws_transit_gateway_route_table_map["me-south-1"]
  peer_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "us-east-1", null)])

  tags = merge(var.default_tags, {
    SkyforgeRole = "tgw-mesh"
  })

  providers = {
    aws.requester = aws.us-east-1
    aws.peer      = aws.me-south-1
  }
}

module "aws_tgw_mesh_eu_central_1__ap_northeast_1" {
  count = local.aws_tgw_mesh_link_eu_central_1__ap_northeast_1 == null ? 0 : 1

  source = "./modules/aws/tgw_peering"

  requester_region            = local.aws_tgw_mesh_link_eu_central_1__ap_northeast_1.source_region
  requester_tgw_id            = local.aws_transit_gateway_map["eu-central-1"]
  requester_route_table_id    = local.aws_transit_gateway_route_table_map["eu-central-1"]
  requester_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "ap-northeast-1", null)])

  peer_region            = local.aws_tgw_mesh_link_eu_central_1__ap_northeast_1.target_region
  peer_tgw_id            = local.aws_transit_gateway_map["ap-northeast-1"]
  peer_route_table_id    = local.aws_transit_gateway_route_table_map["ap-northeast-1"]
  peer_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "eu-central-1", null)])

  tags = merge(var.default_tags, {
    SkyforgeRole = "tgw-mesh"
  })

  providers = {
    aws.requester = aws.eu-central-1
    aws.peer      = aws.ap-northeast-1
  }
}

module "aws_tgw_mesh_eu_central_1__me_south_1" {
  count = local.aws_tgw_mesh_link_eu_central_1__me_south_1 == null ? 0 : 1

  source = "./modules/aws/tgw_peering"

  requester_region            = local.aws_tgw_mesh_link_eu_central_1__me_south_1.source_region
  requester_tgw_id            = local.aws_transit_gateway_map["eu-central-1"]
  requester_route_table_id    = local.aws_transit_gateway_route_table_map["eu-central-1"]
  requester_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "me-south-1", null)])

  peer_region            = local.aws_tgw_mesh_link_eu_central_1__me_south_1.target_region
  peer_tgw_id            = local.aws_transit_gateway_map["me-south-1"]
  peer_route_table_id    = local.aws_transit_gateway_route_table_map["me-south-1"]
  peer_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "eu-central-1", null)])

  tags = merge(var.default_tags, {
    SkyforgeRole = "tgw-mesh"
  })

  providers = {
    aws.requester = aws.eu-central-1
    aws.peer      = aws.me-south-1
  }
}

module "aws_tgw_mesh_ap_northeast_1__me_south_1" {
  count = local.aws_tgw_mesh_link_ap_northeast_1__me_south_1 == null ? 0 : 1

  source = "./modules/aws/tgw_peering"

  requester_region            = local.aws_tgw_mesh_link_ap_northeast_1__me_south_1.source_region
  requester_tgw_id            = local.aws_transit_gateway_map["ap-northeast-1"]
  requester_route_table_id    = local.aws_transit_gateway_route_table_map["ap-northeast-1"]
  requester_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "me-south-1", null)])

  peer_region            = local.aws_tgw_mesh_link_ap_northeast_1__me_south_1.target_region
  peer_tgw_id            = local.aws_transit_gateway_map["me-south-1"]
  peer_route_table_id    = local.aws_transit_gateway_route_table_map["me-south-1"]
  peer_destination_cidrs = compact([lookup(local.aws_region_cidr_blocks, "ap-northeast-1", null)])

  tags = merge(var.default_tags, {
    SkyforgeRole = "tgw-mesh"
  })

  providers = {
    aws.requester = aws.ap-northeast-1
    aws.peer      = aws.me-south-1
  }
}

resource "azurerm_virtual_network_peering" "azure_mesh" {
  for_each                  = local.azure_vnet_peering_map
  name                      = "peer-${replace(each.value.source_region, "-", "")}-${replace(each.value.target_region, "-", "")}"
  resource_group_name       = module.azure_regions[each.value.source_region].resource_group_name
  virtual_network_name      = basename(module.azure_regions[each.value.source_region].virtual_network_ids["shared-services"])
  remote_virtual_network_id = module.azure_regions[each.value.target_region].virtual_network_ids["shared-services"]

  allow_virtual_network_access = true
  allow_forwarded_traffic      = true
  allow_gateway_transit        = true
  use_remote_gateways          = false
}

resource "google_compute_network_peering" "gcp_mesh" {
  provider = google
  for_each = local.gcp_network_mesh_map

  name         = "peer-skyforge-${replace(each.value.source_region, "-", "")}-${replace(each.value.target_region, "-", "")}"
  network      = local.gcp_module_lookup[each.value.source_region].network_ids["shared-services"]
  peer_network = local.gcp_module_lookup[each.value.target_region].network_ids["shared-services"]

  export_custom_routes                = true
  import_custom_routes                = true
  export_subnet_routes_with_public_ip = true
  import_subnet_routes_with_public_ip = true
}

resource "aws_globalaccelerator_accelerator" "global_app" {
  provider = aws.global
  count    = local.aws_global_app_enabled ? 1 : 0

  name            = "skyforge-global-app"
  enabled         = true
  ip_address_type = "IPV4"

  tags = merge(var.default_tags, {
    Name = "skyforge-global-app"
  })
}

resource "aws_globalaccelerator_listener" "global_app" {
  provider = aws.global
  count    = local.aws_global_app_enabled ? 1 : 0

  accelerator_arn = aws_globalaccelerator_accelerator.global_app[0].arn
  protocol        = "TCP"

  dynamic "port_range" {
    for_each = { for idx, port in local.aws_global_app_listener_ports : idx => port }
    content {
      from_port = port_range.value
      to_port   = port_range.value
    }
  }
}

resource "aws_globalaccelerator_endpoint_group" "global_app" {
  provider = aws.global
  for_each = local.aws_global_app_enabled ? local.aws_global_alb_endpoints : {}

  listener_arn          = aws_globalaccelerator_listener.global_app[0].arn
  endpoint_group_region = each.key

  endpoint_configuration {
    endpoint_id = each.value.load_balancer_arn
    weight      = lookup(local.aws_global_app_endpoint_weights, each.key, 100)
  }

  health_check_port             = try(local.aws_global_app_health_check.port, null)
  health_check_protocol         = try(local.aws_global_app_health_check.protocol, null)
  health_check_interval_seconds = try(local.aws_global_app_health_check.interval, null)
  threshold_count               = try(local.aws_global_app_health_check.threshold, null)
}

resource "aws_route53_record" "global_app" {
  provider = aws.us-east-1
  count    = local.aws_global_app_enabled && length(module.dns) > 0 && local.aws_global_app_record_name != "" ? 1 : 0

  zone_id = module.dns[0].zone.zone_id
  name    = local.aws_global_app_record_name
  type    = "CNAME"
  ttl     = 60
  records = [aws_globalaccelerator_accelerator.global_app[0].dns_name]

  depends_on = [aws_globalaccelerator_endpoint_group.global_app]
}

locals {
  aws_module_lookup = {
    "us-east-1"      = try(module.aws_us_east_1[0], null)
    "eu-central-1"   = try(module.aws_eu_central_1[0], null)
    "ap-northeast-1" = try(module.aws_ap_northeast_1[0], null)
    "me-south-1"     = try(module.aws_me_south_1[0], null)
  }

  gcp_module_lookup = {
    "us-central1"     = try(module.gcp_us_central1[0], null)
    "europe-west1"    = try(module.gcp_europe_west1[0], null)
    "asia-southeast1" = try(module.gcp_asia_southeast1[0], null)
    "me-west1"        = try(module.gcp_me_west1[0], null)
  }

  vpn_cloud_links = {
    for link in module.vpn_mesh.cloud_links :
    link.link_id => link
  }

  aws_cloud_links = {
    for link_id, link in local.vpn_cloud_links :
    link_id => link
    if link.source.cloud == "aws" && contains(keys(local.aws_region_configs), link.source.region) && local.aws_region_configs[link.source.region] != null
  }

  aws_to_azure_links = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.target.cloud == "azure" && contains(keys(var.azure_regions), link.target.region)
  }

  aws_to_gcp_links = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.target.cloud == "gcp" && contains(keys(local.gcp_region_configs), link.target.region) && lookup(local.gcp_region_configs, link.target.region, null) != null
  }

  cloud_link_target_ip_map = {
    for link_id, link in local.aws_cloud_links :
    link_id => (
      link.target.cloud == "azure" ?
      try(module.azure_regions[link.target.region].vpn_gateway_public_ips, []) :
      link.target.cloud == "gcp" ?
      try(local.gcp_module_lookup[link.target.region].ha_vpn_gateway_external_ips, []) :
      []
    )
  }

}

resource "random_password" "cloud_link_tunnel1_psk" {
  for_each = local.aws_cloud_links

  length  = 32
  special = false
}

resource "random_password" "cloud_link_tunnel2_psk" {
  for_each = local.aws_cloud_links

  length  = 32
  special = false
}

resource "aws_customer_gateway" "cloud_links_us_east_1" {
  provider = aws.us-east-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "us-east-1"
  }

  bgp_asn    = each.value.bgp.target_asn
  ip_address = local.cloud_link_target_ip_map[each.key][0]
  type       = "ipsec.1"

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-cgw"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_customer_gateway" "cloud_links_eu_central_1" {
  provider = aws.eu-central-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "eu-central-1"
  }

  bgp_asn    = each.value.bgp.target_asn
  ip_address = local.cloud_link_target_ip_map[each.key][0]
  type       = "ipsec.1"

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-cgw"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_customer_gateway" "cloud_links_ap_northeast_1" {
  provider = aws.ap-northeast-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "ap-northeast-1"
  }

  bgp_asn    = each.value.bgp.target_asn
  ip_address = local.cloud_link_target_ip_map[each.key][0]
  type       = "ipsec.1"

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-cgw"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_customer_gateway" "cloud_links_me_south_1" {
  provider = aws.me-south-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "me-south-1"
  }

  bgp_asn    = each.value.bgp.target_asn
  ip_address = local.cloud_link_target_ip_map[each.key][0]
  type       = "ipsec.1"

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-cgw"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_vpn_connection" "cloud_links_us_east_1" {
  provider = aws.us-east-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "us-east-1"
  }

  transit_gateway_id  = local.aws_module_lookup["us-east-1"].transit_gateway_id
  customer_gateway_id = aws_customer_gateway.cloud_links_us_east_1[each.key].id
  type                = "ipsec.1"
  static_routes_only  = false

  tunnel1_preshared_key = format("A%s", random_password.cloud_link_tunnel1_psk[each.key].result)
  tunnel2_preshared_key = format("A%s", random_password.cloud_link_tunnel2_psk[each.key].result)

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-vpn"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_vpn_connection" "cloud_links_eu_central_1" {
  provider = aws.eu-central-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "eu-central-1"
  }

  transit_gateway_id  = local.aws_module_lookup["eu-central-1"].transit_gateway_id
  customer_gateway_id = aws_customer_gateway.cloud_links_eu_central_1[each.key].id
  type                = "ipsec.1"
  static_routes_only  = false

  tunnel1_preshared_key = format("A%s", random_password.cloud_link_tunnel1_psk[each.key].result)
  tunnel2_preshared_key = format("A%s", random_password.cloud_link_tunnel2_psk[each.key].result)

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-vpn"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_vpn_connection" "cloud_links_ap_northeast_1" {
  provider = aws.ap-northeast-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "ap-northeast-1"
  }

  transit_gateway_id  = local.aws_module_lookup["ap-northeast-1"].transit_gateway_id
  customer_gateway_id = aws_customer_gateway.cloud_links_ap_northeast_1[each.key].id
  type                = "ipsec.1"
  static_routes_only  = false

  tunnel1_preshared_key = format("A%s", random_password.cloud_link_tunnel1_psk[each.key].result)
  tunnel2_preshared_key = format("A%s", random_password.cloud_link_tunnel2_psk[each.key].result)

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-vpn"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "aws_vpn_connection" "cloud_links_me_south_1" {
  provider = aws.me-south-1

  for_each = {
    for link_id, link in local.aws_cloud_links :
    link_id => link
    if link.source.region == "me-south-1"
  }

  transit_gateway_id  = local.aws_module_lookup["me-south-1"].transit_gateway_id
  customer_gateway_id = aws_customer_gateway.cloud_links_me_south_1[each.key].id
  type                = "ipsec.1"
  static_routes_only  = false

  tunnel1_preshared_key = format("A%s", random_password.cloud_link_tunnel1_psk[each.key].result)
  tunnel2_preshared_key = format("A%s", random_password.cloud_link_tunnel2_psk[each.key].result)

  tags = merge(var.default_tags, {
    Name         = "skyforge-${each.value.source.region}-${each.value.target.cloud}-${each.key}-vpn"
    SkyforgeRole = "cloud-mesh"
  })
}

locals {
  aws_cloud_link_customer_gateways = merge(
    aws_customer_gateway.cloud_links_us_east_1,
    aws_customer_gateway.cloud_links_eu_central_1,
    aws_customer_gateway.cloud_links_ap_northeast_1,
    aws_customer_gateway.cloud_links_me_south_1
  )

  aws_cloud_link_connections = merge(
    aws_vpn_connection.cloud_links_us_east_1,
    aws_vpn_connection.cloud_links_eu_central_1,
    aws_vpn_connection.cloud_links_ap_northeast_1,
    aws_vpn_connection.cloud_links_me_south_1
  )

  aws_to_azure_tunnel_details = {
    for link_id, link in local.aws_to_azure_links :
    link_id => [
      {
        name               = "aws-tunnel-1"
        outside_address    = local.aws_cloud_link_connections[link_id].tunnel1_address
        vgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel1_vgw_inside_address
        cgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel1_cgw_inside_address
        shared_key         = random_password.cloud_link_tunnel1_psk[link_id].result
        peer_asn           = link.bgp.source_asn
      },
      {
        name               = "aws-tunnel-2"
        outside_address    = local.aws_cloud_link_connections[link_id].tunnel2_address
        vgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel2_vgw_inside_address
        cgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel2_cgw_inside_address
        shared_key         = random_password.cloud_link_tunnel2_psk[link_id].result
        peer_asn           = link.bgp.source_asn
      }
    ]
  }

  aws_to_gcp_tunnel_details = {
    for link_id, link in local.aws_to_gcp_links :
    link_id => {
      tunnel1 = {
        outside_address    = local.aws_cloud_link_connections[link_id].tunnel1_address
        inside_cidr        = local.aws_cloud_link_connections[link_id].tunnel1_inside_cidr
        vgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel1_vgw_inside_address
        cgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel1_cgw_inside_address
        shared_key         = random_password.cloud_link_tunnel1_psk[link_id].result
        peer_asn           = link.bgp.source_asn
      }
      tunnel2 = {
        outside_address    = local.aws_cloud_link_connections[link_id].tunnel2_address
        inside_cidr        = local.aws_cloud_link_connections[link_id].tunnel2_inside_cidr
        vgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel2_vgw_inside_address
        cgw_inside_address = local.aws_cloud_link_connections[link_id].tunnel2_cgw_inside_address
        shared_key         = random_password.cloud_link_tunnel2_psk[link_id].result
        peer_asn           = link.bgp.source_asn
      }
    }
  }

  aws_to_gcp_gateway_interface_ips = {
    for link_id, tunnels in local.aws_to_gcp_tunnel_details :
    link_id => compact([
      tunnels.tunnel1.outside_address,
      tunnels.tunnel2.outside_address
    ])
  }

  azure_cloud_link_sites       = azurerm_vpn_site.cloud_links
  azure_cloud_link_connections = azurerm_vpn_gateway_connection.cloud_links

  gcp_cloud_link_external_gateways = merge(
    google_compute_external_vpn_gateway.cloud_links_us_central1,
    google_compute_external_vpn_gateway.cloud_links_europe_west1,
    google_compute_external_vpn_gateway.cloud_links_asia_southeast1,
    google_compute_external_vpn_gateway.cloud_links_me_west1
  )

  gcp_cloud_link_vpn_tunnels = merge(
    google_compute_vpn_tunnel.cloud_links_us_central1,
    google_compute_vpn_tunnel.cloud_links_europe_west1,
    google_compute_vpn_tunnel.cloud_links_asia_southeast1,
    google_compute_vpn_tunnel.cloud_links_me_west1
  )

  gcp_cloud_link_router_interfaces = merge(
    google_compute_router_interface.cloud_links_us_central1,
    google_compute_router_interface.cloud_links_europe_west1,
    google_compute_router_interface.cloud_links_asia_southeast1,
    google_compute_router_interface.cloud_links_me_west1
  )

  gcp_cloud_link_router_peers = merge(
    google_compute_router_peer.cloud_links_us_central1,
    google_compute_router_peer.cloud_links_europe_west1,
    google_compute_router_peer.cloud_links_asia_southeast1,
    google_compute_router_peer.cloud_links_me_west1
  )
}

resource "azurerm_vpn_site" "cloud_links" {
  for_each = local.aws_to_azure_links

  name                = "vpnsite-skyforge-${each.value.source.region}-to-${each.value.target.region}"
  location            = module.azure_regions[each.value.target.region].location
  resource_group_name = module.azure_regions[each.value.target.region].resource_group_name
  virtual_wan_id      = module.azure_regions[each.value.target.region].virtual_wan_id

  dynamic "link" {
    for_each = [
      for entry in local.aws_to_azure_tunnel_details[each.key] :
      entry if entry.outside_address != null && entry.outside_address != ""
    ]
    content {
      name       = link.value.name
      ip_address = link.value.outside_address

      bgp {
        asn             = link.value.peer_asn
        peering_address = link.value.vgw_inside_address
      }
    }
  }

  tags = merge(var.default_tags, {
    Name         = "vpnsite-skyforge-${each.value.source.region}-to-${each.value.target.region}"
    SkyforgeRole = "cloud-mesh"
  })
}

resource "azurerm_vpn_gateway_connection" "cloud_links" {
  for_each = local.aws_to_azure_links

  name                      = "vpnconn-skyforge-${each.value.source.region}-to-${each.value.target.region}"
  vpn_gateway_id            = module.azure_regions[each.value.target.region].vpn_gateway_id
  remote_vpn_site_id        = azurerm_vpn_site.cloud_links[each.key].id
  internet_security_enabled = false

  dynamic "vpn_link" {
    for_each = [
      for idx, entry in local.aws_to_azure_tunnel_details[each.key] :
      {
        index    = idx
        metadata = entry
      }
      if entry.outside_address != null && entry.outside_address != ""
    ]
    content {
      name             = vpn_link.value.metadata.name
      vpn_site_link_id = azurerm_vpn_site.cloud_links[each.key].link[tonumber(vpn_link.value.index)].id
      protocol         = "IKEv2"
      shared_key       = vpn_link.value.metadata.shared_key
      bgp_enabled      = true
    }
  }
}

resource "google_compute_external_vpn_gateway" "cloud_links_us_central1" {
  provider = google.us_central1

  for_each = {
    for link_id, link in local.aws_to_gcp_links :
    link_id => link
    if link.target.region == "us-central1"
  }

  name        = "ext-gw-skyforge-${each.value.source.region}-to-${each.value.target.region}"
  description = "Skyforge AWS to GCP external gateway (${each.key})"

  redundancy_type = length(local.aws_to_gcp_gateway_interface_ips[each.key]) > 1 ? "TWO_IPS_REDUNDANCY" : "SINGLE_IP_INTERNALLY_REDUNDANT"

  dynamic "interface" {
    for_each = {
      for idx, addr in local.aws_to_gcp_gateway_interface_ips[each.key] :
      tostring(idx) => addr
    }
    content {
      id         = tonumber(interface.key)
      ip_address = interface.value
    }
  }
}

resource "google_compute_vpn_tunnel" "cloud_links_us_central1" {
  provider = google.us_central1

  for_each = google_compute_external_vpn_gateway.cloud_links_us_central1

  name                            = "vpn-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  region                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_gateway                     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].ha_vpn_gateway_self_link
  vpn_gateway_interface           = 0
  peer_external_gateway           = google_compute_external_vpn_gateway.cloud_links_us_central1[each.key].id
  peer_external_gateway_interface = 0
  shared_secret                   = random_password.cloud_link_tunnel1_psk[each.key].result
  ike_version                     = 2
  router                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
}

resource "google_compute_router_interface" "cloud_links_us_central1" {
  provider = google.us_central1

  for_each = google_compute_vpn_tunnel.cloud_links_us_central1

  name       = "if-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_tunnel = google_compute_vpn_tunnel.cloud_links_us_central1[each.key].name
  ip_range = format("%s/%s",
    local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address,
    element(split("/", local.aws_to_gcp_tunnel_details[each.key].tunnel1.inside_cidr), 1)
  )
}

resource "google_compute_router_peer" "cloud_links_us_central1" {
  provider = google.us_central1

  for_each = google_compute_router_interface.cloud_links_us_central1

  name            = "peer-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  interface       = google_compute_router_interface.cloud_links_us_central1[each.key].name
  peer_asn        = local.aws_to_gcp_tunnel_details[each.key].tunnel1.peer_asn
  peer_ip_address = local.aws_to_gcp_tunnel_details[each.key].tunnel1.vgw_inside_address
  ip_address      = local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address
  advertise_mode  = "DEFAULT"
  enable          = true
}

resource "google_compute_external_vpn_gateway" "cloud_links_europe_west1" {
  provider = google.europe_west1

  for_each = {
    for link_id, link in local.aws_to_gcp_links :
    link_id => link
    if link.target.region == "europe-west1"
  }

  name        = "ext-gw-skyforge-${each.value.source.region}-to-${each.value.target.region}"
  description = "Skyforge AWS to GCP external gateway (${each.key})"

  redundancy_type = length(local.aws_to_gcp_gateway_interface_ips[each.key]) > 1 ? "TWO_IPS_REDUNDANCY" : "SINGLE_IP_INTERNALLY_REDUNDANT"

  dynamic "interface" {
    for_each = {
      for idx, addr in local.aws_to_gcp_gateway_interface_ips[each.key] :
      tostring(idx) => addr
    }
    content {
      id         = tonumber(interface.key)
      ip_address = interface.value
    }
  }
}

resource "google_compute_vpn_tunnel" "cloud_links_europe_west1" {
  provider = google.europe_west1

  for_each = google_compute_external_vpn_gateway.cloud_links_europe_west1

  name                            = "vpn-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  region                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_gateway                     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].ha_vpn_gateway_self_link
  vpn_gateway_interface           = 0
  peer_external_gateway           = google_compute_external_vpn_gateway.cloud_links_europe_west1[each.key].id
  peer_external_gateway_interface = 0
  shared_secret                   = random_password.cloud_link_tunnel1_psk[each.key].result
  ike_version                     = 2
  router                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
}

resource "google_compute_router_interface" "cloud_links_europe_west1" {
  provider = google.europe_west1

  for_each = google_compute_vpn_tunnel.cloud_links_europe_west1

  name       = "if-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_tunnel = google_compute_vpn_tunnel.cloud_links_europe_west1[each.key].name
  ip_range = format("%s/%s",
    local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address,
    element(split("/", local.aws_to_gcp_tunnel_details[each.key].tunnel1.inside_cidr), 1)
  )
}

resource "google_compute_router_peer" "cloud_links_europe_west1" {
  provider = google.europe_west1

  for_each = google_compute_router_interface.cloud_links_europe_west1

  name            = "peer-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  interface       = google_compute_router_interface.cloud_links_europe_west1[each.key].name
  peer_asn        = local.aws_to_gcp_tunnel_details[each.key].tunnel1.peer_asn
  peer_ip_address = local.aws_to_gcp_tunnel_details[each.key].tunnel1.vgw_inside_address
  ip_address      = local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address
  advertise_mode  = "DEFAULT"
  enable          = true
}

resource "google_compute_external_vpn_gateway" "cloud_links_asia_southeast1" {
  provider = google.asia_southeast1

  for_each = {
    for link_id, link in local.aws_to_gcp_links :
    link_id => link
    if link.target.region == "asia-southeast1"
  }

  name        = "ext-gw-skyforge-${each.value.source.region}-to-${each.value.target.region}"
  description = "Skyforge AWS to GCP external gateway (${each.key})"

  redundancy_type = length(local.aws_to_gcp_gateway_interface_ips[each.key]) > 1 ? "TWO_IPS_REDUNDANCY" : "SINGLE_IP_INTERNALLY_REDUNDANT"

  dynamic "interface" {
    for_each = {
      for idx, addr in local.aws_to_gcp_gateway_interface_ips[each.key] :
      tostring(idx) => addr
    }
    content {
      id         = tonumber(interface.key)
      ip_address = interface.value
    }
  }
}

resource "google_compute_vpn_tunnel" "cloud_links_asia_southeast1" {
  provider = google.asia_southeast1

  for_each = google_compute_external_vpn_gateway.cloud_links_asia_southeast1

  name                            = "vpn-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  region                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_gateway                     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].ha_vpn_gateway_self_link
  vpn_gateway_interface           = 0
  peer_external_gateway           = google_compute_external_vpn_gateway.cloud_links_asia_southeast1[each.key].id
  peer_external_gateway_interface = 0
  shared_secret                   = random_password.cloud_link_tunnel1_psk[each.key].result
  ike_version                     = 2
  router                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
}

resource "google_compute_router_interface" "cloud_links_asia_southeast1" {
  provider = google.asia_southeast1

  for_each = google_compute_vpn_tunnel.cloud_links_asia_southeast1

  name       = "if-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_tunnel = google_compute_vpn_tunnel.cloud_links_asia_southeast1[each.key].name
  ip_range = format("%s/%s",
    local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address,
    element(split("/", local.aws_to_gcp_tunnel_details[each.key].tunnel1.inside_cidr), 1)
  )
}

resource "google_compute_router_peer" "cloud_links_asia_southeast1" {
  provider = google.asia_southeast1

  for_each = google_compute_router_interface.cloud_links_asia_southeast1

  name            = "peer-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  interface       = google_compute_router_interface.cloud_links_asia_southeast1[each.key].name
  peer_asn        = local.aws_to_gcp_tunnel_details[each.key].tunnel1.peer_asn
  peer_ip_address = local.aws_to_gcp_tunnel_details[each.key].tunnel1.vgw_inside_address
  ip_address      = local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address
  advertise_mode  = "DEFAULT"
  enable          = true
}

resource "google_compute_external_vpn_gateway" "cloud_links_me_west1" {
  provider = google.me_west1

  for_each = {
    for link_id, link in local.aws_to_gcp_links :
    link_id => link
    if link.target.region == "me-west1"
  }

  name        = "ext-gw-skyforge-${each.value.source.region}-to-${each.value.target.region}"
  description = "Skyforge AWS to GCP external gateway (${each.key})"

  redundancy_type = length(local.aws_to_gcp_gateway_interface_ips[each.key]) > 1 ? "TWO_IPS_REDUNDANCY" : "SINGLE_IP_INTERNALLY_REDUNDANT"

  dynamic "interface" {
    for_each = {
      for idx, addr in local.aws_to_gcp_gateway_interface_ips[each.key] :
      tostring(idx) => addr
    }
    content {
      id         = tonumber(interface.key)
      ip_address = interface.value
    }
  }
}

resource "google_compute_vpn_tunnel" "cloud_links_me_west1" {
  provider = google.me_west1

  for_each = google_compute_external_vpn_gateway.cloud_links_me_west1

  name                            = "vpn-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  region                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_gateway                     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].ha_vpn_gateway_self_link
  vpn_gateway_interface           = 0
  peer_external_gateway           = google_compute_external_vpn_gateway.cloud_links_me_west1[each.key].id
  peer_external_gateway_interface = 0
  shared_secret                   = random_password.cloud_link_tunnel1_psk[each.key].result
  ike_version                     = 2
  router                          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
}

resource "google_compute_router_interface" "cloud_links_me_west1" {
  provider = google.me_west1

  for_each = google_compute_vpn_tunnel.cloud_links_me_west1

  name       = "if-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region     = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  vpn_tunnel = google_compute_vpn_tunnel.cloud_links_me_west1[each.key].name
  ip_range = format("%s/%s",
    local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address,
    element(split("/", local.aws_to_gcp_tunnel_details[each.key].tunnel1.inside_cidr), 1)
  )
}

resource "google_compute_router_peer" "cloud_links_me_west1" {
  provider = google.me_west1

  for_each = google_compute_router_interface.cloud_links_me_west1

  name            = "peer-skyforge-${local.aws_to_gcp_links[each.key].source.region}-to-${local.aws_to_gcp_links[each.key].target.region}"
  router          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.name
  region          = local.gcp_module_lookup[local.aws_to_gcp_links[each.key].target.region].default_router.region
  interface       = google_compute_router_interface.cloud_links_me_west1[each.key].name
  peer_asn        = local.aws_to_gcp_tunnel_details[each.key].tunnel1.peer_asn
  peer_ip_address = local.aws_to_gcp_tunnel_details[each.key].tunnel1.vgw_inside_address
  ip_address      = local.aws_to_gcp_tunnel_details[each.key].tunnel1.cgw_inside_address
  advertise_mode  = "DEFAULT"
  enable          = true
}

output "aws_transit_gateways" {
  description = "Transit Gateway IDs by AWS region."
  value       = local.aws_transit_gateway_map
}

output "azure_virtual_hubs" {
  description = "Virtual WAN hub IDs by Azure region."
  value = {
    for k, mod in module.azure_regions :
    k => mod.virtual_wan_hub_id
  }
}

output "azure_firewalls" {
  description = "Azure Firewall resources by region (when enabled)."
  value = {
    for k, mod in module.azure_regions :
    k => {
      firewall_id        = mod.firewall_id
      firewall_public_ip = mod.firewall_public_ip
    }
  }
}

output "gcp_ha_vpn_gateways" {
  description = "HA VPN gateway self links by region."
  value       = local.gcp_ha_vpn_map
}

output "vpn_endpoint_manifest" {
  description = "Rendered VPN endpoint manifest."
  value       = module.vnfs.vpn_manifest
  sensitive   = true
}

output "dns_zone" {
  description = "Route53 DNS zone details when configured."
  value       = length(module.dns) > 0 ? module.dns[0].zone : null
}

output "multi_cloud_load_balancing" {
  description = "Summary of multi-cloud load balancing resources."
  value = {
    aws = {
      transit_gateway_connect = local.aws_transit_gateway_connect_map
      global_accelerators     = local.aws_global_accelerator_map
      application_albs        = local.aws_global_alb_endpoints
      gateway_load_balancers  = local.aws_gwlb_map
      global_application_accelerator = local.aws_global_app_enabled ? {
        dns_name         = aws_globalaccelerator_accelerator.global_app[0].dns_name
        endpoint_regions = keys(local.aws_global_alb_endpoints)
        listener_ports   = local.aws_global_app_listener_ports
        health_check     = local.aws_global_app_health_check
        custom_domain    = length(aws_route53_record.global_app) > 0 ? aws_route53_record.global_app[0].fqdn : null
        endpoint_weights = local.aws_global_app_endpoint_weights
        ip_sets          = [for s in aws_globalaccelerator_accelerator.global_app[0].ip_sets : s.ip_addresses]
      } : null
    }
    azure = {
      front_door = local.azure_front_door_map
      asa        = local.azure_asa_map
    }
    gcp = {
      global_http_load_balancers = local.gcp_global_lb_map
      checkpoint_firewalls       = local.gcp_checkpoint_map
    }
  }
  sensitive = true
}

output "reachability" {
  description = "Summary of configured reachability and connectivity tests."
  value = {
    aws = {
      paths    = local.aws_reachability_paths
      analyses = local.aws_reachability_analyses
    }
    azure = local.azure_reachability_map
    gcp   = local.gcp_reachability_map
  }
}
