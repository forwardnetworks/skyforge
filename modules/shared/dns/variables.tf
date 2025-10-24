variable "domain_name" {
  description = "Root domain name for the Route53 zone."
  type        = string
}

variable "comment" {
  description = "Descriptive comment for the zone."
  type        = string
  default     = ""
}

variable "private_zone" {
  description = "Whether to create a private hosted zone."
  type        = bool
  default     = false
}

variable "vpc_associations" {
  description = "List of VPC associations for private zones."
  type = list(object({
    vpc_id = string
    region = string
  }))
  default = []
}

variable "tags" {
  description = "Tags applied to the hosted zone."
  type        = map(string)
  default     = {}
}

variable "records" {
  description = "Optional DNS records to seed the zone with."
  type = map(object({
    name   = optional(string)
    type   = string
    ttl    = optional(number, 300)
    values = optional(list(string), [])
    alias = optional(object({
      name                   = string
      zone_id                = string
      evaluate_target_health = optional(bool, false)
    }))
  }))
  default = {}
}
