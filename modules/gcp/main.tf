locals {
  name_prefix         = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
  name_prefix_compact = var.resource_suffix == "" ? "skyforge" : format("skyforge%s", var.resource_suffix)
  vpc_map             = var.region_config.vpcs
  default_vpc_key     = contains(keys(var.region_config.vpcs), "shared-services") ? "shared-services" : sort(keys(var.region_config.vpcs))[0]

  subnet_blueprints = flatten([
    for vpc_name, vpc in local.vpc_map : [
      for tier_index, tier in vpc.tier_labels : [
        for offset in range(vpc.subnet_count) : {
          key      = "${vpc_name}-${tier}-${offset}"
          vpc_name = vpc_name
          tier     = tier
          offset   = offset
          cidr     = cidrsubnet(vpc.cidr_block, vpc.subnet_prefix_extension, tier_index * vpc.subnet_count + offset)
        }
      ]
    ]
  ])

  subnet_map = { for blueprint in local.subnet_blueprints : blueprint.key => blueprint }
}

resource "google_compute_network" "this" {
  for_each                = local.vpc_map
  name                    = "${local.name_prefix}-${var.region_key}-${each.key}"
  auto_create_subnetworks = false
  routing_mode            = each.value.routing_mode
}

resource "google_compute_subnetwork" "this" {
  for_each = local.subnet_map

  name          = format("subnet-%s-%s-%02d", each.value.vpc_name, each.value.tier, each.value.offset)
  region        = var.region_config.region
  network       = google_compute_network.this[each.value.vpc_name].self_link
  ip_cidr_range = each.value.cidr
}

locals {
  subnet_ids_by_tier = {
    for key, subnet in google_compute_subnetwork.this :
    "${local.subnet_map[key].vpc_name}.${local.subnet_map[key].tier}" => subnet.id...
  }

  subnet_id_map = {
    for tier_key, ids in local.subnet_ids_by_tier :
    tier_key => ids[0]
  }

  network_id_map = {
    for name, network in google_compute_network.this :
    name => network.id
  }

  workloads_enabled  = var.region_config.workloads != null
  workloads_config   = try(var.region_config.workloads, null)
  checkpoint_config  = try(var.region_config.checkpoint_firewall, null)
  checkpoint_enabled = local.checkpoint_config != null && try(local.checkpoint_config.enable, true)
  checkpoint_subnet_candidates = local.checkpoint_enabled ? [
    for key, subnet in local.subnet_map :
    key if subnet.vpc_name == local.checkpoint_config.vpc_name && subnet.tier == local.checkpoint_config.subnet_tier
  ] : []
  checkpoint_subnet_key       = length(local.checkpoint_subnet_candidates) > 0 ? local.checkpoint_subnet_candidates[0] : null
  checkpoint_subnet_self_link = local.checkpoint_subnet_key != null ? google_compute_subnetwork.this[local.checkpoint_subnet_key].self_link : null
  checkpoint_metadata_base    = local.checkpoint_enabled ? merge(try(local.checkpoint_config.metadata, {}), {}) : {}
  checkpoint_startup_script   = local.checkpoint_enabled ? try(local.checkpoint_config.startup_script, null) : null
}

resource "google_compute_firewall" "tier_allow" {
  for_each = { for vpc_name, vpc in local.vpc_map : vpc_name => vpc if vpc.create_firewall }

  name    = format("fw-%s-%s-%s-allow", local.name_prefix, var.region_key, each.key)
  network = google_compute_network.this[each.key].name

  allow {
    protocol = "tcp"
    ports    = ["80", "443", "22", "3389"]
  }

  source_ranges = [var.region_config.cidr_block]
  target_tags   = [format("%s-%s", local.name_prefix, each.key)]
}

resource "google_compute_router" "this" {
  for_each = local.vpc_map

  name    = "cr-${local.name_prefix}-${var.region_key}-${each.key}"
  network = google_compute_network.this[each.key].self_link
  region  = var.region_config.region

  bgp {
    asn = 64514 + index(sort(keys(local.vpc_map)), each.key)
  }
}

resource "google_compute_router_nat" "this" {
  for_each = var.region_config.enable_cloud_natt ? { for router_name in keys(local.vpc_map) : router_name => router_name } : {}

  name   = "nat-${local.name_prefix}-${var.region_key}-${each.key}"
  router = google_compute_router.this[each.key].name
  region = var.region_config.region

  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

resource "google_compute_ha_vpn_gateway" "this" {
  count   = var.region_config.enable_ha_vpn ? 1 : 0
  name    = "ha-vpn-${local.name_prefix}-${var.region_key}"
  network = google_compute_network.this[local.default_vpc_key].self_link
  region  = var.region_config.region
}

locals {
  ha_vpn_enabled     = var.region_config.enable_ha_vpn
  gcp_vnf_link_order = local.ha_vpn_enabled ? [for link in var.vnf_links : link.site if try(link.customer_gateway_ipv4, null) != null] : []
  gcp_vnf_link_map = local.ha_vpn_enabled ? {
    for link in var.vnf_links :
    link.site => merge(link, {
      endpoint_ip = try(link.customer_gateway_ipv4, null)
    })
    if try(link.customer_gateway_ipv4, null) != null
  } : {}
  gcp_vnf_link_ranges = {
    for site in local.gcp_vnf_link_order :
    site => cidrsubnet("169.254.200.0/24", 6, index(local.gcp_vnf_link_order, site))
  }
}

module "workloads" {
  count = local.workloads_enabled ? 1 : 0

  source = "./workloads"

  providers = {
    google      = google
    google-beta = google-beta
  }

  region_key      = var.region_key
  region_config   = var.region_config
  default_tags    = var.default_tags
  subnet_id_map   = local.subnet_id_map
  network_id_map  = local.network_id_map
  resource_suffix = var.resource_suffix

  // ensure networks/subnets exist before creating workload resources
  depends_on = [
    google_compute_network.this,
    google_compute_subnetwork.this
  ]
}

resource "random_password" "vnf_shared_secret" {
  for_each = local.gcp_vnf_link_map

  length  = 32
  special = false
}

resource "random_password" "checkpoint_admin" {
  count = local.checkpoint_enabled && try(local.checkpoint_config.admin_password, null) == null ? 1 : 0

  length  = 20
  special = false
}

resource "google_compute_external_vpn_gateway" "vnf" {
  for_each = local.gcp_vnf_link_map

  name            = format("ext-gw-%s-%s-%s", local.name_prefix, var.region_key, each.key)
  redundancy_type = "SINGLE_IP_INTERNALLY_REDUNDANT"
  description     = "Skyforge external VNF gateway for ${each.key}"

  interface {
    id         = 0
    ip_address = each.value.endpoint_ip
  }

  labels = {
    for k, v in var.default_tags :
    lower(k) => v
  }
}

resource "google_compute_vpn_tunnel" "vnf" {
  for_each = local.gcp_vnf_link_map

  name                            = format("vpn-%s-%s-%s", local.name_prefix, var.region_key, each.key)
  region                          = var.region_config.region
  vpn_gateway                     = google_compute_ha_vpn_gateway.this[0].id
  vpn_gateway_interface           = 0
  peer_external_gateway           = google_compute_external_vpn_gateway.vnf[each.key].id
  peer_external_gateway_interface = 0
  shared_secret                   = random_password.vnf_shared_secret[each.key].result
  ike_version                     = 2
  router                          = google_compute_router.this[local.default_vpc_key].name

  labels = {
    for k, v in var.default_tags :
    lower(k) => v
  }
}

resource "google_compute_router_interface" "vnf" {
  for_each = local.gcp_vnf_link_map

  name       = format("if-%s-%s-%s", local.name_prefix, var.region_key, each.key)
  router     = google_compute_router.this[local.default_vpc_key].name
  region     = var.region_config.region
  vpn_tunnel = google_compute_vpn_tunnel.vnf[each.key].name
  ip_range   = local.gcp_vnf_link_ranges[each.key]
}

resource "google_compute_router_peer" "vnf" {
  for_each = local.gcp_vnf_link_map

  name            = format("peer-%s-%s-%s", local.name_prefix, var.region_key, each.key)
  router          = google_compute_router.this[local.default_vpc_key].name
  region          = var.region_config.region
  interface       = google_compute_router_interface.vnf[each.key].name
  peer_asn        = each.value.bgp_asn
  peer_ip_address = cidrhost(local.gcp_vnf_link_ranges[each.key], 2)
  ip_address      = cidrhost(local.gcp_vnf_link_ranges[each.key], 1)
}

locals {
  checkpoint_admin_username = local.checkpoint_enabled ? try(local.checkpoint_config.admin_username, "cpadmin") : null
  checkpoint_admin_password = local.checkpoint_enabled ? coalesce(try(local.checkpoint_config.admin_password, null), try(random_password.checkpoint_admin[0].result, null)) : null
}

resource "google_compute_instance" "checkpoint" {
  count        = local.checkpoint_enabled && local.checkpoint_subnet_self_link != null ? 1 : 0
  name         = format("cp-%s-firewall", var.region_key)
  machine_type = try(local.checkpoint_config.machine_type, "n2-standard-4")
  zone         = format("%s-a", var.region_config.region)

  boot_disk {
    initialize_params {
      image = try(local.checkpoint_config.source_image, "projects/ubuntu-os-cloud/global/images/family/ubuntu-2004-lts")
      size  = try(local.checkpoint_config.disk_size_gb, 50)
      type  = try(local.checkpoint_config.disk_type, "pd-balanced")
    }
  }

  network_interface {
    subnetwork = local.checkpoint_subnet_self_link
  }

  metadata = merge(
    {
      vendor         = "checkpoint"
      admin-username = local.checkpoint_admin_username
      admin-password = local.checkpoint_admin_password
    },
    local.checkpoint_metadata_base,
    local.checkpoint_startup_script != null ? {
      "startup-script" = local.checkpoint_startup_script
    } : {}
  )

  tags = concat([format("%s-checkpoint", local.name_prefix), format("%s-%s", local.name_prefix, var.region_key)], try(local.checkpoint_config.tags, []))

  dynamic "service_account" {
    for_each = compact([try(local.checkpoint_config.service_account, null)])
    content {
      email  = service_account.value
      scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    }
  }

  labels = {
    for k, v in var.default_tags :
    lower(k) => v
  }

  depends_on = [google_compute_subnetwork.this]
}


locals {
  vpn_manifest = {
    region      = var.region_config.region
    cidr        = var.region_config.cidr_block
    ipv6_prefix = var.region_config.ipv6_prefix
    ha_vpn      = try(google_compute_ha_vpn_gateway.this[0].self_link, null)
    vpcs = {
      for name, network in google_compute_network.this :
      name => {
        id              = network.id
        self_link       = network.self_link
        routing_mode    = local.vpc_map[name].routing_mode
        create_firewall = local.vpc_map[name].create_firewall
        subnets = [
          for key, subnet in google_compute_subnetwork.this :
          {
            id   = subnet.id
            tier = local.subnet_map[key].tier
            cidr = local.subnet_map[key].cidr
          } if local.subnet_map[key].vpc_name == name
        ]
      }
    }
  }
}
