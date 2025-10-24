variable "region_key" {
  description = "Key of the GCP region where the connectivity test is defined."
  type        = string
}

variable "project_id" {
  description = "GCP project ID used for the connectivity tests."
  type        = string
  default     = null
}

variable "tests" {
  description = "Connectivity tests to create for the region."
  type = list(object({
    name             = string
    source_ip        = string
    destination_ip   = string
    protocol         = optional(string, "TCP")
    destination_port = optional(number, 443)
    source_port      = optional(number)
    description      = optional(string)
    related_projects = optional(list(string), [])
    enabled          = optional(bool, true)
  }))
  default = []
}

variable "labels" {
  description = "Labels applied to the connectivity test resources."
  type        = map(string)
  default     = {}
}
