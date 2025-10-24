variable "sites" {
  description = "Virtual network function endpoint definitions keyed by friendly name."
  type = map(object({
    location         = string
    ipv4_cidr        = string
    ipv6_prefix      = string
    device_type      = string
    tunnel_count     = number
    connect_to_cloud = optional(bool, true)
    vpn_gateway_ipv6 = optional(string)
    vpn_gateway_ipv4 = optional(string)
  }))
}

variable "output_filename" {
  description = "Path (relative to the root module) where the VPN manifest JSON will be written."
  type        = string
  default     = "outputs/vpn-endpoints.json"
}

variable "cloud_manifests" {
  description = "Aggregated cloud-side VPN endpoint metadata keyed by cloud provider."
  type = object({
    aws   = map(any)
    azure = map(any)
    gcp   = map(any)
    mesh = optional(object({
      cloud_links = list(any)
      vnf_links   = list(any)
      link_status = map(any)
      }), {
      cloud_links = []
      vnf_links   = []
      link_status = {}
    })
  })
  default = {
    aws   = {}
    azure = {}
    gcp   = {}
    mesh = {
      cloud_links = []
      vnf_links   = []
      link_status = {}
    }
  }
}
