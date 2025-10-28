################################################################################
# GCP Workload Module                                                          #
################################################################################

# Provides optional application-layer components on top of the networking
# foundation. Each block under `workloads` in the region configuration is
# optional; resources are only created when explicitly requested.

data "google_client_config" "current" {}

locals {
  project_id          = coalesce(try(var.region_config.project_id, null), data.google_client_config.current.project)
  cfg                 = try(var.region_config.workloads, {})
  name_prefix         = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
  name_prefix_compact = var.resource_suffix == "" ? "skyforge" : format("skyforge%s", var.resource_suffix)

  gke_cfg       = try(local.cfg.gke, null)
  armor_cfg     = try(local.cfg.cloud_armor, null)
  storage_cfg   = try(local.cfg.storage, null)
  run_cfg       = try(local.cfg.cloud_run, null)
  function_cfg  = try(local.cfg.cloud_function, null)
  pubsub_cfg    = try(local.cfg.pubsub, null)
  sql_cfg       = try(local.cfg.sql, null)
  global_lb_cfg = try(local.cfg.global_lb, null)

  gke_enabled                     = local.gke_cfg != null
  armor_enabled                   = local.armor_cfg != null
  storage_enabled                 = local.storage_cfg != null
  run_enabled                     = local.run_cfg != null
  function_enabled                = local.function_cfg != null
  pubsub_enabled                  = local.pubsub_cfg != null
  sql_enabled                     = local.sql_cfg != null
  global_lb_enabled               = local.global_lb_cfg != null
  global_lb_backend_type          = local.global_lb_enabled ? lower(try(local.global_lb_cfg.backend_type, "cloud_run")) : null
  global_lb_backend_is_cloud_run  = local.global_lb_enabled && local.global_lb_backend_type == "cloud_run"
  global_lb_backend_is_storage    = local.global_lb_enabled && local.global_lb_backend_type == "storage_bucket"
  global_lb_backend_can_cloud_run = local.global_lb_backend_is_cloud_run && local.run_enabled
  global_lb_backend_can_storage   = local.global_lb_backend_is_storage && local.storage_enabled
  global_lb_ready                 = local.global_lb_enabled && (local.global_lb_backend_can_cloud_run || local.global_lb_backend_can_storage)

  gke_subnet_key = local.gke_enabled ? "${local.gke_cfg.vpc_name}.${local.gke_cfg.subnet_tier}" : null
  gke_subnet_id  = local.gke_enabled ? lookup(var.subnet_id_map, local.gke_subnet_key, null) : null
  gke_network_id = local.gke_enabled ? lookup(var.network_id_map, local.gke_cfg.vpc_name, null) : null

  base_labels = {
    for k, v in var.default_tags :
    lower(k) => v
  }
}

################################################################################
# Google Kubernetes Engine                                                     #
################################################################################

resource "google_container_cluster" "this" {
  count    = local.gke_enabled ? 1 : 0
  project  = local.project_id
  name     = "${local.name_prefix}-${var.region_key}-gke"
  location = var.region_config.region

  network    = local.gke_network_id
  subnetwork = local.gke_subnet_id

  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = try(local.gke_cfg.deletion_protection, false)

  release_channel {
    channel = try(local.gke_cfg.release_channel, "REGULAR")
  }

  lifecycle {
    precondition {
      condition     = local.gke_network_id != null && local.gke_subnet_id != null
      error_message = "GKE workload enabled but VPC/subnet mapping could not be resolved."
    }
  }

  resource_labels = merge(local.base_labels, {
    component = "gke"
  })
}

resource "google_container_node_pool" "this" {
  count      = local.gke_enabled ? 1 : 0
  project    = local.project_id
  name       = "${local.name_prefix}-${var.region_key}-pool"
  location   = var.region_config.region
  cluster    = google_container_cluster.this[0].name
  node_count = try(local.gke_cfg.node_count, 2)

  node_config {
    machine_type = try(local.gke_cfg.node_machine, "e2-standard-2")
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]
    labels       = merge(local.base_labels, { component = "gke" })
    disk_size_gb = try(local.gke_cfg.disk_size_gb, 100)
    disk_type    = try(local.gke_cfg.disk_type, "pd-balanced")
  }
}

################################################################################
# Cloud Armor                                                                  #
################################################################################

resource "google_compute_security_policy" "this" {
  count       = local.armor_enabled ? 1 : 0
  project     = local.project_id
  name        = "${local.name_prefix}-${var.region_key}-armor"
  description = "Skyforge baseline Cloud Armor policy"

  rule {
    priority = 1000
    action   = try(local.armor_cfg.action, "ALLOW")
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["0.0.0.0/0"]
      }
    }
  }
}

################################################################################
# Cloud Storage                                                                #
################################################################################

resource "random_id" "bucket_suffix" {
  count       = local.storage_enabled ? 1 : 0
  byte_length = 4
}

resource "google_storage_bucket" "this" {
  count         = local.storage_enabled ? 1 : 0
  name          = lower(format("%s-%s-%s", local.name_prefix, var.region_key, random_id.bucket_suffix[0].hex))
  project       = local.project_id
  location      = try(local.storage_cfg.location, "US")
  force_destroy = true
  labels        = merge(local.base_labels, { component = "storage" })
}

################################################################################
# Cloud Run                                                                     #
################################################################################

resource "google_cloud_run_service" "this" {
  provider                   = google-beta
  count                      = local.run_enabled ? 1 : 0
  project                    = local.project_id
  name                       = coalesce(try(local.run_cfg.service_name, null), format("%s-%s-run", local.name_prefix, var.region_key))
  location                   = var.region_config.region
  autogenerate_revision_name = true

  template {
    metadata {
      labels = merge(local.base_labels, { component = "cloud-run" })
    }
    spec {
      containers {
        image = local.run_cfg.image
        ports {
          container_port = try(local.run_cfg.port, 8080)
        }
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }
}

resource "google_cloud_run_service_iam_member" "public_invoker" {
  provider = google-beta
  count    = local.run_enabled && try(local.run_cfg.allow_unauthenticated, true) ? 1 : 0
  project  = local.project_id
  location = google_cloud_run_service.this[0].location
  service  = google_cloud_run_service.this[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

################################################################################
# Cloud Functions (1st gen)                                                     #
################################################################################

resource "google_cloudfunctions_function" "this" {
  count = local.function_enabled ? 1 : 0

  project               = local.project_id
  name                  = local.function_cfg.name
  runtime               = try(local.function_cfg.runtime, "python310")
  entry_point           = try(local.function_cfg.entry_point, "function")
  region                = var.region_config.region
  source_archive_bucket = local.function_cfg.source_archive_bucket
  source_archive_object = local.function_cfg.source_archive_object
  trigger_http          = try(local.function_cfg.trigger_http, true)
  available_memory_mb   = 128
}

resource "google_cloudfunctions_function_iam_member" "public_invoker" {
  count          = local.function_enabled && try(local.function_cfg.trigger_http, true) ? 1 : 0
  project        = local.project_id
  region         = var.region_config.region
  cloud_function = google_cloudfunctions_function.this[0].name
  role           = "roles/cloudfunctions.invoker"
  member         = "allUsers"
}

################################################################################
# Pub/Sub                                                                       #
################################################################################

resource "google_pubsub_topic" "this" {
  count   = local.pubsub_enabled ? 1 : 0
  project = local.project_id
  name    = local.pubsub_cfg.topic_name
  labels  = merge(local.base_labels, { component = "pubsub" })
}

resource "google_pubsub_subscription" "this" {
  count   = local.pubsub_enabled ? 1 : 0
  project = local.project_id
  name    = local.pubsub_cfg.subscription_name
  topic   = google_pubsub_topic.this[0].name
}

################################################################################
# Cloud SQL                                                                     #
################################################################################

resource "random_password" "sql" {
  count   = local.sql_enabled ? 1 : 0
  length  = 20
  special = true
}

resource "google_sql_database_instance" "this" {
  count            = local.sql_enabled ? 1 : 0
  project          = local.project_id
  name             = "${local.name_prefix}-${var.region_key}-sql"
  region           = try(local.sql_cfg.region, var.region_config.region)
  database_version = try(local.sql_cfg.database_version, "POSTGRES_15")

  settings {
    tier              = try(local.sql_cfg.tier, "db-custom-2-7680")
    availability_type = "ZONAL"
    ip_configuration {
      ipv4_enabled = true
    }
  }

  deletion_protection = false
}

resource "google_sql_database" "this" {
  count    = local.sql_enabled ? 1 : 0
  project  = local.project_id
  name     = "appdb"
  instance = google_sql_database_instance.this[0].name
}

resource "google_sql_user" "this" {
  count    = local.sql_enabled ? 1 : 0
  project  = local.project_id
  instance = google_sql_database_instance.this[0].name
  name     = "appuser"
  password = random_password.sql[0].result
}

################################################################################
# Global External HTTP Load Balancer                                          #
################################################################################

resource "google_compute_region_network_endpoint_group" "cloud_run" {
  count                 = local.global_lb_backend_can_cloud_run ? 1 : 0
  project               = local.project_id
  name                  = "neg-${local.name_prefix}-${var.region_key}"
  region                = var.region_config.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.this[0].name
  }

  depends_on = [google_cloud_run_service.this]
}

resource "google_compute_backend_service" "global" {
  count                 = local.global_lb_backend_can_cloud_run ? 1 : 0
  project               = local.project_id
  name                  = "${local.name_prefix}-${var.region_key}-backend"
  protocol              = "HTTP"
  port_name             = "http"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  timeout_sec           = 30
  enable_cdn            = try(local.global_lb_cfg.enable_cdn, false)

  backend {
    group = google_compute_region_network_endpoint_group.cloud_run[0].self_link
  }

  depends_on = [google_compute_region_network_endpoint_group.cloud_run]
}

resource "google_compute_backend_bucket" "global" {
  count       = local.global_lb_backend_can_storage ? 1 : 0
  project     = local.project_id
  name        = "${local.name_prefix}-${var.region_key}-backend"
  bucket_name = google_storage_bucket.this[0].name
  enable_cdn  = try(local.global_lb_cfg.enable_cdn, false)
}

resource "google_compute_url_map" "global" {
  count       = local.global_lb_ready ? 1 : 0
  project     = local.project_id
  name        = "${local.name_prefix}-${var.region_key}-urlmap"
  description = try(local.global_lb_cfg.description, null)

  default_service = local.global_lb_backend_can_cloud_run ? google_compute_backend_service.global[0].self_link : google_compute_backend_bucket.global[0].self_link
}

resource "google_compute_target_http_proxy" "global" {
  count   = local.global_lb_ready ? 1 : 0
  project = local.project_id
  name    = "${local.name_prefix}-${var.region_key}-http-proxy"
  url_map = google_compute_url_map.global[0].self_link
}

resource "google_compute_global_address" "global" {
  count      = local.global_lb_ready ? 1 : 0
  project    = local.project_id
  name       = "${local.name_prefix}-${var.region_key}-glb"
  ip_version = "IPV4"
}

resource "google_compute_global_forwarding_rule" "global" {
  count                 = local.global_lb_ready ? 1 : 0
  project               = local.project_id
  name                  = "${local.name_prefix}-${var.region_key}-http-fr"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  port_range            = "80"
  target                = google_compute_target_http_proxy.global[0].self_link
  ip_address            = google_compute_global_address.global[0].address
  labels                = merge(local.base_labels, { component = "global-lb" })
}

################################################################################
# Metadata export                                                               #
################################################################################

locals {
  metadata = {
    gke = local.gke_enabled ? {
      cluster_name = google_container_cluster.this[0].name
      endpoint     = google_container_cluster.this[0].endpoint
    } : null
    cloud_armor = local.armor_enabled ? {
      policy_name = google_compute_security_policy.this[0].name
    } : null
    storage = local.storage_enabled ? {
      bucket = google_storage_bucket.this[0].name
    } : null
    cloud_run = local.run_enabled ? {
      service_name = google_cloud_run_service.this[0].name
      url          = google_cloud_run_service.this[0].status[0].url
    } : null
    cloud_function = local.function_enabled ? {
      name      = google_cloudfunctions_function.this[0].name
      https_url = google_cloudfunctions_function.this[0].https_trigger_url
    } : null
    pubsub = local.pubsub_enabled ? {
      topic        = google_pubsub_topic.this[0].name
      subscription = google_pubsub_subscription.this[0].name
    } : null
    sql = local.sql_enabled ? {
      instance = google_sql_database_instance.this[0].name
      database = google_sql_database.this[0].name
      user     = google_sql_user.this[0].name
    } : null
    global_load_balancer = local.global_lb_ready ? {
      forwarding_rule = google_compute_global_forwarding_rule.global[0].name
      ip_address      = google_compute_global_address.global[0].address
      backend_type    = local.global_lb_backend_type
    } : null
  }
}
