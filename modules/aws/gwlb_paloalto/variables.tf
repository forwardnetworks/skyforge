variable "vpc_id" {
  type        = string
  description = "VPC where the Gateway Load Balancer and firewalls are deployed."
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets used by the Gateway Load Balancer and firewall instances."
}

variable "tags" {
  type        = map(string)
  description = "Tags applied to created resources."
  default     = {}
}

variable "firewall_ami_id" {
  type        = string
  description = "Palo Alto VM-Series AMI ID. When null, firewall instances are not launched."
  default     = null
}

variable "instance_type" {
  type        = string
  description = "Instance type for Palo Alto appliances."
  default     = "m5.xlarge"
}

variable "instance_count" {
  type        = number
  description = "Number of firewall instances to launch."
  default     = 2
}

variable "iam_instance_profile" {
  type        = string
  description = "Optional IAM instance profile for the firewall instances."
  default     = null
}

variable "user_data" {
  type        = string
  description = "Optional user data for firewall bootstrapping."
  default     = null
}

variable "bootstrap" {
  description = "Optional structured bootstrap definition used when explicit user_data is not supplied."
  type = object({
    hostname       = optional(string, "skyforge-palo")
    admin_username = optional(string, "skyforgeadmin")
    admin_password = optional(string, "Skyforge!Pass123")
    dns_servers    = optional(list(string), ["1.1.1.1", "8.8.8.8"])
    ntp_servers    = optional(list(string), ["time.google.com", "pool.ntp.org"])
    log_profile    = optional(string, "Skyforge-Log-Profile")
    auth_code      = optional(string)
    security_policies = optional(list(object({
      name                  = string
      description           = optional(string, "")
      source_zones          = optional(list(string), ["trust"])
      destination_zones     = optional(list(string), ["untrust"])
      source_addresses      = optional(list(string), ["any"])
      destination_addresses = optional(list(string), ["any"])
      applications          = optional(list(string), ["any"])
      services              = optional(list(string), ["application-default"])
      action                = optional(string, "allow")
      log_setting           = optional(string)
    })), [])
    address_objects = optional(list(object({
      name        = string
      type        = optional(string, "ip-netmask")
      value       = string
      description = optional(string, "")
    })), [])
    service_objects = optional(list(object({
      name             = string
      protocol         = string
      destination_port = string
      source_port      = optional(string, "any")
      description      = optional(string, "")
    })), [])
  })
  default = null
}

variable "name_prefix" {
  type        = string
  description = "Prefix applied to GWLB and firewall resource names."
  default     = "skyforge-palo"
}

variable "allowed_principals" {
  type        = list(string)
  description = "Optional list of principals allowed to create GWLB endpoints."
  default     = []
}
