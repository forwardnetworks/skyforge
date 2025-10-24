variable "region_key" {
  description = "Region key for naming consistency."
  type        = string
}

variable "location" {
  description = "Azure region location string."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group to deploy workloads into."
  type        = string
}

variable "default_tags" {
  description = "Base tag set from parent module."
  type        = map(string)
  default     = {}
}

variable "config" {
  description = "Workload configuration for AKS/App Gateway/App Service/SQL."
  type        = any
}

variable "subnet_id_map" {
  description = "Map of <vnet>.<tier> => subnet ID for lookup."
  type        = map(string)
}

variable "vnet_id_map" {
  description = "Map of VNet name to ID for workload references."
  type        = map(string)
}

variable "firewall_private_ip" {
  description = "Azure Firewall private IP (optional for next hop)."
  type        = string
  default     = null
}
