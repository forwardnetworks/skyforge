data "aws_ami" "fortinet" {
  count       = var.ami_id == null && var.ami_product_code != null ? 1 : 0
  most_recent = true
  owners      = ["aws-marketplace"]

  filter {
    name   = "product-code"
    values = [var.ami_product_code]
  }
}

locals {
  tag_base = merge(var.tags, var.tags_extra, {
    SkyforgeComponent = "fortinet-connect"
  })

  inside_ip      = cidrhost(var.inside_cidr, 2)
  inside_netmask = cidrnetmask(var.inside_cidr)
  tgw_inside_ip  = cidrhost(var.inside_cidr, 1)

  ami_id_resolved = coalesce(
    var.ami_id,
    try(data.aws_ami.fortinet[0].image_id, null)
  )
}

resource "random_password" "admin" {
  count = var.admin_password == null ? 1 : 0

  length  = 20
  special = false
}

locals {
  admin_username_effective = var.admin_username
  admin_password_effective = coalesce(var.admin_password, try(random_password.admin[0].result, null))

  rendered_user_data = var.user_data != null ? var.user_data : templatefile("${path.module}/templates/fortinet_user_data.tpl", {
    hostname            = format("%s-fgt", var.name_prefix)
    inside_ip           = local.inside_ip
    inside_netmask      = local.inside_netmask
    inside_cidr         = var.inside_cidr
    tgw_inside_ip       = local.tgw_inside_ip
    peer_bgp_asn        = var.peer_bgp_asn
    transit_gateway_asn = var.transit_gateway_asn
    advertised_prefixes = var.advertised_prefixes
    admin_username      = local.admin_username_effective
    admin_password      = local.admin_password_effective
  })
}

resource "aws_security_group" "fortinet" {
  name        = "${var.name_prefix}-sg"
  description = "Skyforge Fortinet TGW Connect security group"
  vpc_id      = var.vpc_id

  ingress {
    description = "Management SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.management_cidrs
  }

  ingress {
    description = "HTTPS management"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = var.management_cidrs
  }

  ingress {
    description = "FortiGate GUI"
    from_port   = 8443
    to_port     = 8443
    protocol    = "tcp"
    cidr_blocks = var.management_cidrs
  }

  ingress {
    description = "BGP from TGW overlay"
    from_port   = 179
    to_port     = 179
    protocol    = "tcp"
    cidr_blocks = [var.inside_cidr]
  }

  ingress {
    description = "GRE from Transit Gateway"
    from_port   = 0
    to_port     = 0
    protocol    = "47"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "IKE/ISAKMP"
    from_port   = 500
    to_port     = 500
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "IPsec NAT-T"
    from_port   = 4500
    to_port     = 4500
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tag_base, {
    Name = "${var.name_prefix}-sg"
  })
}

resource "aws_instance" "fortinet" {
  ami                         = local.ami_id_resolved
  instance_type               = var.instance_type
  subnet_id                   = var.subnet_id
  associate_public_ip_address = false
  vpc_security_group_ids      = [aws_security_group.fortinet.id]
  iam_instance_profile        = var.iam_instance_profile
  key_name                    = var.key_name
  user_data                   = local.rendered_user_data
  user_data_replace_on_change = true
  source_dest_check           = false

  metadata_options {
    http_tokens = "required"
  }

  lifecycle {
    precondition {
      condition     = local.ami_id_resolved != null
      error_message = "Fortinet connector requires either ami_id or a resolvable ami_product_code."
    }
  }

  tags = merge(local.tag_base, {
    Name = "${var.name_prefix}-fortinet"
  })
}

output "instance_id" {
  description = "EC2 instance ID for the Fortinet appliance."
  value       = aws_instance.fortinet.id
}

output "private_ip" {
  description = "Private IP address of the Fortinet appliance (used as TGW Connect peer address)."
  value       = aws_instance.fortinet.private_ip
}

output "management_ip" {
  description = "Management IP address for the Fortinet appliance."
  value       = aws_instance.fortinet.private_ip
}

output "security_group_id" {
  description = "Security group protecting the Fortinet appliance."
  value       = aws_security_group.fortinet.id
}

output "inside_ip" {
  description = "Fortinet inside GRE/BGP peer address."
  value       = local.inside_ip
}

output "tgw_inside_ip" {
  description = "Transit Gateway inside address used for BGP peering."
  value       = local.tgw_inside_ip
}

output "admin_credentials" {
  description = "Administrative credentials for the Fortinet appliance."
  value = {
    username = local.admin_username_effective
    password = local.admin_password_effective
  }
  sensitive = true
}
