locals {
  deployment_subnet_ids = length(var.subnet_ids) >= 2 ? slice(var.subnet_ids, 0, 2) : var.subnet_ids
  instance_count        = length(local.deployment_subnet_ids)
}

resource "aws_security_group" "this" {
  name        = "${var.name_prefix}-sg"
  description = "Security group for ${var.name_prefix} firewall"
  vpc_id      = var.vpc_id

  ingress {
    description = "Allow all ingress for appliance traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-sg"
  })
}

resource "aws_instance" "this" {
  count = local.instance_count

  ami                         = var.ami_id
  instance_type               = var.instance_type
  subnet_id                   = element(local.deployment_subnet_ids, count.index)
  associate_public_ip_address = false
  vpc_security_group_ids      = [aws_security_group.this.id]
  iam_instance_profile        = var.iam_instance_profile
  user_data                   = var.user_data

  tags = merge(var.tags, {
    Name  = format("%s-%02d", var.name_prefix, count.index + 1)
    Index = tostring(count.index)
  })
}

output "instance_ids" {
  description = "IDs of firewall instances."
  value       = aws_instance.this[*].id
}

output "private_ips" {
  description = "Private IPs of firewall instances."
  value       = aws_instance.this[*].private_ip
}
