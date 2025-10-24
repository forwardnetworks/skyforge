variable "name" {
  description = "Name of the Azure resource group."
  type        = string
}

variable "location" {
  description = "Azure location for the resource group (required when create_if_missing is true)."
  type        = string
  default     = null
}

variable "create_if_missing" {
  description = "Whether to create the resource group when it does not already exist."
  type        = bool
  default     = false
}

variable "tags" {
  description = "Tags to apply if the resource group is created."
  type        = map(string)
  default     = {}
}
