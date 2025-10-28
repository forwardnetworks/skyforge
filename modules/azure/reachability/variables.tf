variable "region_key" {
  description = "Azure region key (friendly name) for tagging."
  type        = string
}

variable "location" {
  description = "Azure location where the network watcher resides."
  type        = string
}

variable "network_watcher_id" {
  description = "ID of the Azure Network Watcher to associate with the connection monitors."
  type        = string
}

variable "tests" {
  description = "Connectivity tests to create for the region."
  type = list(object({
    name                = string
    source_address      = string
    destination_address = string
    protocol            = optional(string, "Tcp")
    destination_port    = optional(number, 443)
    frequency_seconds   = optional(number, 60)
    enabled             = optional(bool, true)
    description         = optional(string)
    trace_route_enabled = optional(bool, true)
  }))
  default = []
}

variable "tags" {
  description = "Tags applied to connection monitor resources."
  type        = map(string)
  default     = {}
}

variable "resource_suffix" {
  description = "Optional suffix appended to connectivity test names."
  type        = string
  default     = ""
}
