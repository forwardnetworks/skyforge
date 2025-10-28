variable "region_key" {
  type        = string
  description = "Friendly region key for tagging."
}

variable "config" {
  description = "Application stack configuration object."
  type        = any
}

variable "default_tags" {
  description = "Base tags applied to created resources."
  type        = map(string)
}

variable "resource_suffix" {
  description = "Optional timestamp suffix appended to names."
  type        = string
  default     = ""
}

variable "frontend_vpc_id" {
  type        = string
  description = "VPC hosting the frontend/ingress resources."
}

variable "app_vpc_id" {
  type        = string
  description = "VPC hosting the mid-tier workloads."
}

variable "data_vpc_id" {
  type        = string
  description = "VPC hosting the data tier workloads."
}

variable "frontend_subnet_ids" {
  type        = list(string)
  description = "Subnets used by the load balancer."
}

variable "app_subnet_ids" {
  type        = list(string)
  description = "Subnets used by application workloads/EKS."
}

variable "data_subnet_ids" {
  type        = list(string)
  description = "Subnets used by database resources."
}
