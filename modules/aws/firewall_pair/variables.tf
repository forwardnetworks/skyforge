variable "name_prefix" {
  description = "Prefix applied to firewall resources."
  type        = string
}

variable "vpc_id" {
  description = "Target VPC identifier."
  type        = string
}

variable "subnet_ids" {
  description = "Subnets for firewall instances."
  type        = list(string)
}

variable "ami_id" {
  description = "AMI used for firewall instances."
  type        = string
}

variable "instance_type" {
  description = "Instance type used for firewall appliances."
  type        = string
  default     = "c6i.large"
}

variable "iam_instance_profile" {
  description = "Optional IAM instance profile name."
  type        = string
  default     = null
}

variable "user_data" {
  description = "Optional user data for firewall bootstrap."
  type        = string
  default     = null
}

variable "tags" {
  description = "Tags applied to firewall resources."
  type        = map(string)
  default     = {}
}
