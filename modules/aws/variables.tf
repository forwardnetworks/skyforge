variable "region_key" {
  description = "Lookup key for the AWS region (friendly name)."
  type        = string
}

variable "region_config" {
  description = "Regional configuration data structure for AWS resources."
  type        = any
}

variable "default_tags" {
  description = "Default tags applied to AWS resources."
  type        = map(string)
  default     = {}
}

variable "vnf_links" {
  description = "VNF VPN link definitions targeting this AWS region."
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

variable "resource_suffix" {
  description = "Optional suffix appended to all nameable AWS resources."
  type        = string
  default     = ""
}
