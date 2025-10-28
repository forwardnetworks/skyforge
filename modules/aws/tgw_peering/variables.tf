variable "requester_region" {
  description = "AWS region of the requester Transit Gateway."
  type        = string
}

variable "peer_region" {
  description = "AWS region of the peer Transit Gateway."
  type        = string
}

variable "requester_tgw_id" {
  description = "Transit Gateway ID in the requester region."
  type        = string
}

variable "peer_tgw_id" {
  description = "Transit Gateway ID in the peer region."
  type        = string
}

variable "requester_route_table_id" {
  description = "Transit Gateway route table ID in the requester region."
  type        = string
}

variable "peer_route_table_id" {
  description = "Transit Gateway route table ID in the peer region."
  type        = string
}

variable "requester_destination_cidrs" {
  description = "CIDR blocks that should route from the requester TGW toward the peer TGW."
  type        = list(string)
  default     = []
}

variable "peer_destination_cidrs" {
  description = "CIDR blocks that should route from the peer TGW toward the requester TGW."
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Common tags to apply to peering resources."
  type        = map(string)
  default     = {}
}

variable "resource_suffix" {
  description = "Optional suffix appended to TGW peering resource names."
  type        = string
  default     = ""
}
