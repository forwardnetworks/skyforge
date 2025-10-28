variable "global_name" {
  description = "Name assigned to the AWS Network Manager global network."
  type        = string
}

variable "description" {
  description = "Optional description for the global network."
  type        = string
  default     = null
}

variable "tags" {
  description = "Base tags applied to Network Manager resources."
  type        = map(string)
  default     = {}
}

variable "sites" {
  description = "List of sites to register with the global network."
  type = list(object({
    name        = string
    description = optional(string)
    region      = optional(string)
    address     = optional(string)
    latitude    = optional(number)
    longitude   = optional(number)
  }))
  default = []
}

variable "devices" {
  description = "Optional list of devices associated with registered sites."
  type = list(object({
    name        = string
    site_name   = string
    type        = optional(string)
    description = optional(string)
    model       = optional(string)
    serial      = optional(string)
  }))
  default = []
}

variable "resource_suffix" {
  description = "Optional suffix to keep global network resource names unique."
  type        = string
  default     = ""
}
