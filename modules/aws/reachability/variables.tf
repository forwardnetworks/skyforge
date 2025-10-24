variable "region_key" {
  description = "AWS region key where reachability resources will be created."
  type        = string
}

variable "paths" {
  description = "Reachability Analyzer path definitions."
  type = list(object({
    name             = string
    description      = optional(string)
    source_vpc       = string
    destination_vpc  = string
    protocol         = optional(string, "TCP")
    source_port      = optional(number)
    destination_port = optional(number)
    perform_analysis = optional(bool, true)
  }))
  default = []
}

variable "transit_gateway_attachments" {
  description = "Map of Transit Gateway attachment IDs keyed by VPC name."
  type        = map(string)
  default     = {}
}

variable "tags" {
  description = "Base tags applied to reachability resources."
  type        = map(string)
  default     = {}
}
