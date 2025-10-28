variable "region_key" {
  description = "Lookup key for the Azure region (friendly name)."
  type        = string
}

variable "region_config" {
  description = "Regional configuration block for Azure resources."
  type        = any
}

variable "default_tags" {
  description = "Default tag map for Azure resources."
  type        = map(string)
  default     = {}
}

variable "vnf_links" {
  description = "VNF VPN link definitions targeting this Azure region."
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

variable "resource_group_name" {
  description = "Optional override for the Azure resource group name. When null, a per-region RG is created."
  type        = string
  default     = null
}

variable "resource_suffix" {
  description = "Optional suffix appended to Azure resource names to avoid collisions."
  type        = string
  default     = ""
}
