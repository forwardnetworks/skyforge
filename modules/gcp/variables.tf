variable "region_key" {
  description = "Lookup key for the GCP region (friendly name)."
  type        = string
}

variable "region_config" {
  description = "Regional configuration for GCP networking."
  type = object({
    region            = string
    project_id        = optional(string)
    cidr_block        = string
    ipv6_prefix       = string
    enable_ha_vpn     = bool
    enable_cloud_natt = bool
    vpcs = map(object({
      cidr_block              = string
      routing_mode            = string
      subnet_count            = number
      create_firewall         = bool
      subnet_prefix_extension = number
      tier_labels             = list(string)
    }))
    workloads = optional(object({
      gke = optional(object({
        vpc_name        = string
        subnet_tier     = string
        node_count      = optional(number, 2)
        node_machine    = optional(string, "e2-standard-2")
        release_channel = optional(string, "REGULAR")
      }))
      cloud_armor = optional(object({
        action = optional(string, "ALLOW")
      }))
      storage = optional(object({
        location = optional(string, "US")
      }))
      cloud_run = optional(object({
        service_name          = optional(string)
        image                 = string
        port                  = optional(number, 8080)
        allow_unauthenticated = optional(bool, true)
      }))
      cloud_function = optional(object({
        name                  = string
        runtime               = optional(string, "python310")
        entry_point           = optional(string, "function")
        source_archive_bucket = string
        source_archive_object = string
        trigger_http          = optional(bool, true)
      }))
      pubsub = optional(object({
        topic_name        = string
        subscription_name = string
      }))
      sql = optional(object({
        database_version = optional(string, "POSTGRES_15")
        tier             = optional(string, "db-custom-2-7680")
        region           = optional(string)
      }))
    }), null)
    checkpoint_firewall = optional(object({
      enable          = optional(bool, true)
      vpc_name        = string
      subnet_tier     = string
      machine_type    = optional(string, "n2-standard-4")
      source_image    = optional(string, "projects/ubuntu-os-cloud/global/images/family/ubuntu-2004-lts")
      disk_size_gb    = optional(number, 50)
      disk_type       = optional(string, "pd-balanced")
      admin_username  = optional(string, "cpadmin")
      admin_password  = optional(string)
      tags            = optional(list(string), [])
      service_account = optional(string)
      startup_script  = optional(string)
      metadata        = optional(map(string), {})
    }), null)
  })
}

variable "default_tags" {
  description = "Key/value metadata applied as labels where supported."
  type        = map(string)
  default     = {}
}

variable "vnf_links" {
  description = "VNF VPN link definitions targeting this GCP region."
  type = list(object({
    site                  = string
    hub_id                = string
    bgp_asn               = number
    tunnel_mode           = string
    preferred_proto       = string
    customer_gateway_ipv4 = optional(string)
    customer_gateway_ipv6 = optional(string)
  }))
  default = []
}

variable "vnf_sites" {
  description = "Map of VNF site metadata keyed by site name."
  type        = map(any)
  default     = {}
}
