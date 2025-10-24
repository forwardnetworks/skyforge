locals {
  firewall_launch_count = var.firewall_ami_id == null ? 0 : var.instance_count
  subnet_cycle_length   = max(length(var.subnet_ids), 1)
  tag_base = merge(var.tags, {
    SkyforgeComponent = "paloalto-gwlb"
  })
  bootstrap_enabled = var.user_data == null && var.bootstrap != null
  bootstrap_address_objects = local.bootstrap_enabled ? [
    for obj in try(var.bootstrap.address_objects, []) : {
      name        = obj.name
      type        = try(obj.type, "ip-netmask")
      value       = obj.value
      description = try(obj.description, "")
    }
  ] : []
  bootstrap_service_objects = local.bootstrap_enabled ? [
    for svc in try(var.bootstrap.service_objects, []) : {
      name             = svc.name
      protocol         = svc.protocol
      destination_port = svc.destination_port
      source_port      = try(svc.source_port, "any")
      description      = try(svc.description, "")
    }
  ] : []
  bootstrap_security_policies = local.bootstrap_enabled ? [
    for rule in try(var.bootstrap.security_policies, []) : {
      name                  = rule.name
      description           = try(rule.description, "")
      source_zones          = coalesce(try(rule.source_zones, null), ["trust"])
      destination_zones     = coalesce(try(rule.destination_zones, null), ["untrust"])
      source_addresses      = coalesce(try(rule.source_addresses, null), ["any"])
      destination_addresses = coalesce(try(rule.destination_addresses, null), ["any"])
      applications          = coalesce(try(rule.applications, null), ["any"])
      services              = coalesce(try(rule.services, null), ["application-default"])
      action                = try(rule.action, "allow")
      log_setting           = try(rule.log_setting, null)
    }
  ] : []
  bootstrap_context = local.bootstrap_enabled ? {
    hostname          = try(var.bootstrap.hostname, "skyforge-palo")
    admin_username    = try(var.bootstrap.admin_username, "skyforgeadmin")
    admin_password    = try(var.bootstrap.admin_password, "Skyforge!Pass123")
    dns_servers       = coalesce(try(var.bootstrap.dns_servers, null), ["1.1.1.1", "8.8.8.8"])
    ntp_servers       = coalesce(try(var.bootstrap.ntp_servers, null), ["time.google.com", "pool.ntp.org"])
    log_profile       = try(var.bootstrap.log_profile, "Skyforge-Log-Profile")
    auth_code         = try(var.bootstrap.auth_code, null)
    address_objects   = local.bootstrap_address_objects
    service_objects   = local.bootstrap_service_objects
    security_policies = local.bootstrap_security_policies
  } : null
  rendered_user_data = var.user_data != null ? var.user_data : (local.bootstrap_enabled ? templatefile("${path.module}/templates/bootstrap.tpl", local.bootstrap_context) : null)
}

resource "aws_security_group" "paloalto" {
  name        = "${var.name_prefix}-sg"
  description = "Security group for Palo Alto GWLB appliances"
  vpc_id      = var.vpc_id

  ingress {
    description = "Allow intra-VPC communication"
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

  tags = merge(local.tag_base, {
    Name = "${var.name_prefix}-sg"
  })
}

resource "aws_lb" "this" {
  name               = substr("${var.name_prefix}-gwlb", 0, 32)
  load_balancer_type = "gateway"

  dynamic "subnet_mapping" {
    for_each = var.subnet_ids
    content {
      subnet_id = subnet_mapping.value
    }
  }

  tags = merge(local.tag_base, {
    Name = substr("${var.name_prefix}-gwlb", 0, 32)
  })
}

resource "aws_lb_target_group" "this" {
  name        = substr("${var.name_prefix}-tg", 0, 32)
  port        = 6081
  protocol    = "GENEVE"
  target_type = "ip"
  vpc_id      = var.vpc_id

  health_check {
    port     = "80"
    protocol = "TCP"
  }

  tags = merge(local.tag_base, {
    Name = substr("${var.name_prefix}-tg", 0, 32)
  })
}

resource "aws_instance" "firewall" {
  count                       = local.firewall_launch_count
  ami                         = var.firewall_ami_id
  instance_type               = coalesce(var.instance_type, "m5.xlarge")
  subnet_id                   = element(var.subnet_ids, count.index % local.subnet_cycle_length)
  associate_public_ip_address = false
  vpc_security_group_ids      = [aws_security_group.paloalto.id]
  iam_instance_profile        = var.iam_instance_profile
  user_data                   = local.rendered_user_data

  lifecycle {
    ignore_changes = [user_data]
  }

  tags = merge(local.tag_base, {
    Name = format("%s-fw-%02d", var.name_prefix, count.index + 1)
  })
}

locals {
  firewall_targets = {
    for idx, instance in aws_instance.firewall :
    idx => instance.private_ip
  }

  firewall_instance_ids = [for instance in aws_instance.firewall : instance.id]
  firewall_private_ips  = [for instance in aws_instance.firewall : instance.private_ip]
  firewall_admin_username = coalesce(
    local.bootstrap_enabled ? try(local.bootstrap_context.admin_username, null) : null,
    try(var.bootstrap.admin_username, null),
    "admin"
  )
  firewall_admin_password = local.bootstrap_enabled ? try(local.bootstrap_context.admin_password, null) : try(var.bootstrap.admin_password, null)
}

resource "aws_lb_target_group_attachment" "firewalls" {
  for_each          = local.firewall_targets
  target_group_arn  = aws_lb_target_group.this.arn
  target_id         = each.value
  port              = 6081
  availability_zone = null
}

resource "aws_vpc_endpoint_service" "this" {
  acceptance_required        = length(var.allowed_principals) > 0
  gateway_load_balancer_arns = [aws_lb.this.arn]
  allowed_principals         = var.allowed_principals

  tags = merge(local.tag_base, {
    Name = substr("${var.name_prefix}-vpces", 0, 32)
  })
}
