variable "mesh" {
  description = "Mesh connectivity definition across clouds and VNF endpoints."
  type = object({
    cloud_links = list(object({
      source = object({
        cloud  = string
        region = string
        hub_id = string
      })
      target = object({
        cloud  = string
        region = string
        hub_id = string
      })
      bgp = object({
        source_asn = number
        target_asn = number
      })
      tunnels = list(object({
        id              = string
        preferred_proto = string
        fallback_proto  = optional(string)
        source_endpoint = string
        target_endpoint = string
      }))
    }))
    vnf_links = list(object({
      site                  = string
      cloud                 = string
      region                = string
      hub_id                = string
      bgp_asn               = number
      tunnel_mode           = string
      preferred_proto       = string
      customer_gateway_ipv6 = optional(string)
      customer_gateway_ipv4 = optional(string)
    }))
  })
}

variable "sites" {
  description = "Resolved VNF endpoint details used to enrich mesh data."
  type        = map(any)
  default     = {}
}
