variable "region_key" {
  description = "GCP region key (friendly name)."
  type        = string
}

variable "region_config" {
  description = "Full region configuration (for location)."
  type        = any
}

variable "default_tags" {
  description = "Default labels/tags for resources."
  type        = map(string)
  default     = {}
}

variable "subnet_id_map" {
  description = "Map of <vpc>.<tier> to subnet ID."
  type        = map(string)
}

variable "network_id_map" {
  description = "Map of VPC name to network ID."
  type        = map(string)
}
