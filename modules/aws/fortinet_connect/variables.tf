variable "vpc_id" {
  description = "VPC where the Fortinet appliance will be launched."
  type        = string
}

variable "subnet_id" {
  description = "Subnet hosting the Fortinet appliance (should be part of the Transit Gateway Connect transport VPC)."
  type        = string
}

variable "tags" {
  description = "Base tags applied to created resources."
  type        = map(string)
  default     = {}
}

variable "tags_extra" {
  description = "Additional tags merged with the defaults."
  type        = map(string)
  default     = {}
}

variable "name_prefix" {
  description = "Prefix applied to Fortinet resources."
  type        = string
  default     = "skyforge-fortinet"
}

variable "ami_id" {
  description = "Optional Fortinet FortiGate AMI identifier. If omitted, the module resolves the latest marketplace image via the provided product code."
  type        = string
  default     = null
}

variable "ami_product_code" {
  description = "Marketplace product code used to discover the Fortinet AMI when ami_id is not provided."
  type        = string
  default     = "cyxc6cynd1msb41uz9fou0byi"
}

variable "instance_type" {
  description = "Instance type for the Fortinet appliance."
  type        = string
  default     = "c5.xlarge"
}

variable "iam_instance_profile" {
  description = "Optional IAM instance profile for the Fortinet instance."
  type        = string
  default     = null
}

variable "key_name" {
  description = "Optional SSH key for Fortinet access."
  type        = string
  default     = null
}

variable "inside_cidr" {
  description = "Transit Gateway Connect inside CIDR (used for the GRE overlay and BGP peering)."
  type        = string
}

variable "peer_bgp_asn" {
  description = "BGP ASN assigned to the Fortinet appliance."
  type        = number
}

variable "transit_gateway_asn" {
  description = "Transit Gateway BGP ASN (remote AS)."
  type        = number
  default     = 64512
}

variable "advertised_prefixes" {
  description = "List of IPv4 prefixes the Fortinet appliance should advertise to the Transit Gateway."
  type        = list(string)
  default     = []
}

variable "management_cidrs" {
  description = "CIDR blocks allowed to reach the Fortinet appliance for administrative access."
  type        = list(string)
  default     = ["10.0.0.0/8", "192.168.0.0/16"]
}

variable "user_data" {
  description = "Optional custom user data to bootstrap the Fortinet appliance. Overrides the generated template when supplied."
  type        = string
  default     = null
}

variable "admin_username" {
  description = "Administrative username configured on the Fortinet appliance."
  type        = string
  default     = "fortinetadmin"
}

variable "admin_password" {
  description = "Optional administrative password for the Fortinet appliance. When omitted, a random password is generated."
  type        = string
  default     = null
}
