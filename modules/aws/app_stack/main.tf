locals {
  name_prefix_base = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
  create_eks       = try(var.config.create_eks, true)
  create_alb       = try(var.config.create_alb, true)
  create_rds       = try(var.config.create_rds, true)
  create_app_asg   = try(var.config.create_app_asg, true)
  create_ga        = local.create_alb && try(var.config.create_global_accelerator, false)

  eks_defaults = {
    name_prefix        = format("%s-%s", local.name_prefix_base, var.region_key)
    version            = "1.29"
    node_instance_type = "t3.medium"
    desired_capacity   = 2
    max_capacity       = 3
    min_capacity       = 1
  }
  eks_config = merge(local.eks_defaults, try(var.config.eks, {}))

  rds_defaults = {
    engine            = "postgres"
    engine_version    = "13.11"
    instance_class    = "db.t3.micro"
    allocated_storage = 20
    database_name     = "appdb"
    username          = "appuser"
  }
  rds_config = merge(local.rds_defaults, try(var.config.rds, {}))

  asg_defaults = {
    desired_capacity = 2
    max_size         = 3
    min_size         = 1
    instance_type    = "t3.micro"
  }
  asg_config = merge(local.asg_defaults, try(var.config.asg, {}))

  base_tags = merge(var.default_tags, {
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "application"
  })
  name_prefix = local.name_prefix_base
}


# -------------------------------------------------------------
# Application load balancer
# -------------------------------------------------------------
resource "aws_security_group" "alb" {
  count = local.create_alb ? 1 : 0

  name        = "${local.name_prefix}-${var.region_key}-alb-sg"
  description = "ALB security group"
  vpc_id      = var.frontend_vpc_id

  ingress {
    description = "Allow HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "Allow HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-alb-sg"
  })
}

resource "aws_lb" "app" {
  count = local.create_alb ? 1 : 0

  name               = "${local.name_prefix}-${var.region_key}-alb"
  load_balancer_type = "application"
  security_groups    = aws_security_group.alb.*.id
  subnets            = var.frontend_subnet_ids

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-alb"
  })
}

resource "aws_lb_target_group" "app" {
  count = local.create_alb ? 1 : 0

  name     = "${local.name_prefix}-${var.region_key}-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = var.app_vpc_id

  health_check {
    enabled  = true
    interval = 30
    path     = "/"
    port     = "traffic-port"
    protocol = "HTTP"
  }

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-tg"
  })
}

resource "aws_lb_listener" "app_http" {
  count = local.create_alb ? 1 : 0

  load_balancer_arn = aws_lb.app[0].arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app[0].arn
  }
}

# -------------------------------------------------------------
# AWS Global Accelerator (optional)
# -------------------------------------------------------------
resource "aws_globalaccelerator_accelerator" "app" {
  count = local.create_ga ? 1 : 0

  name            = "${local.name_prefix}-${var.region_key}-ga"
  enabled         = true
  ip_address_type = "IPV4"

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-ga"
  })
}

resource "aws_globalaccelerator_listener" "app" {
  count = local.create_ga ? 1 : 0

  accelerator_arn = aws_globalaccelerator_accelerator.app[0].arn
  protocol        = "TCP"

  port_range {
    from_port = 80
    to_port   = 80
  }
}

resource "aws_globalaccelerator_endpoint_group" "app" {
  count = local.create_ga ? 1 : 0

  listener_arn          = aws_globalaccelerator_listener.app[0].arn
  endpoint_group_region = var.region_key

  endpoint_configuration {
    endpoint_id = aws_lb.app[0].arn
  }
}

# -------------------------------------------------------------
# Application compute tier
# -------------------------------------------------------------
resource "aws_security_group" "app" {
  count = local.create_app_asg ? 1 : 0

  name        = "${local.name_prefix}-${var.region_key}-app-sg"
  description = "Application tier security group"
  vpc_id      = var.app_vpc_id

  ingress {
    description = "Allow HTTP from ALB"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-app-sg"
  })
}

resource "aws_iam_role" "app_instance" {
  count = local.create_app_asg ? 1 : 0

  name = "${local.name_prefix}-${var.region_key}-app-instance-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_instance_profile" "app_instance" {
  count = local.create_app_asg ? 1 : 0

  name = "${local.name_prefix}-${var.region_key}-app-profile"
  role = aws_iam_role.app_instance[0].name
}

data "aws_ssm_parameter" "amazon_linux_2" {
  count = local.create_app_asg ? 1 : 0

  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

resource "aws_launch_template" "app" {
  count = local.create_app_asg ? 1 : 0

  name_prefix   = "${local.name_prefix}-${var.region_key}-app-lt"
  image_id      = data.aws_ssm_parameter.amazon_linux_2[0].value
  instance_type = local.asg_config.instance_type
  iam_instance_profile {
    name = aws_iam_instance_profile.app_instance[0].name
  }
  user_data = base64encode(<<-EOT
              #!/bin/bash
              echo "<h1>Skyforge App - ${var.region_key}</h1>" > /var/www/html/index.html
              dnf install -y nginx
              systemctl enable nginx
              systemctl start nginx
              EOT
  )

  vpc_security_group_ids = aws_security_group.app.*.id

  tag_specifications {
    resource_type = "instance"
    tags = merge(local.base_tags, {
      Name = "${local.name_prefix}-${var.region_key}-app"
    })
  }
}

resource "aws_autoscaling_group" "app" {
  count = local.create_app_asg ? 1 : 0

  name                = "${local.name_prefix}-${var.region_key}-app-asg"
  desired_capacity    = local.asg_config.desired_capacity
  max_size            = local.asg_config.max_size
  min_size            = local.asg_config.min_size
  vpc_zone_identifier = var.app_subnet_ids
  health_check_type   = "EC2"
  launch_template {
    id      = aws_launch_template.app[0].id
    version = "$Latest"
  }
  tag {
    key                 = "Name"
    value               = "${local.name_prefix}-${var.region_key}-app"
    propagate_at_launch = true
  }
}

resource "aws_autoscaling_attachment" "app" {
  count = local.create_app_asg && local.create_alb ? 1 : 0

  autoscaling_group_name = aws_autoscaling_group.app[0].name
  lb_target_group_arn    = aws_lb_target_group.app[0].arn
}

# -------------------------------------------------------------
# RDS PostgreSQL instance
# -------------------------------------------------------------
resource "aws_security_group" "rds" {
  count = local.create_rds ? 1 : 0

  name        = "${local.name_prefix}-${var.region_key}-rds-sg"
  description = "Database security group"
  vpc_id      = var.data_vpc_id

  ingress {
    description     = "Postgres from app tier"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = aws_security_group.app.*.id
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-rds-sg"
  })
}

resource "aws_db_subnet_group" "this" {
  count = local.create_rds ? 1 : 0

  name       = "${local.name_prefix}-${var.region_key}-rds-subnet"
  subnet_ids = var.data_subnet_ids

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-rds-subnet"
  })
}

resource "random_password" "rds" {
  count = local.create_rds ? 1 : 0

  length  = 16
  special = true
}

resource "aws_db_instance" "this" {
  count = local.create_rds ? 1 : 0

  identifier             = "${local.name_prefix}-${var.region_key}-rds"
  allocated_storage      = local.rds_config.allocated_storage
  db_subnet_group_name   = aws_db_subnet_group.this[0].name
  engine                 = local.rds_config.engine
  engine_version         = local.rds_config.engine_version
  instance_class         = local.rds_config.instance_class
  username               = local.rds_config.username
  password               = random_password.rds[0].result
  db_name                = local.rds_config.database_name
  vpc_security_group_ids = aws_security_group.rds.*.id
  skip_final_snapshot    = true
  publicly_accessible    = false
  multi_az               = false
  storage_encrypted      = true
  deletion_protection    = false
  apply_immediately      = true
}

# -------------------------------------------------------------
# Amazon EKS cluster + node group
# -------------------------------------------------------------
resource "aws_security_group" "eks_cluster" {
  count = local.create_eks ? 1 : 0

  name        = "${local.name_prefix}-${var.region_key}-eks-cluster-sg"
  description = "EKS cluster communication"
  vpc_id      = var.app_vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-eks-cluster-sg"
  })
}

resource "aws_security_group" "eks_nodes" {
  count = local.create_eks ? 1 : 0

  name        = "${local.name_prefix}-${var.region_key}-eks-nodes-sg"
  description = "EKS worker nodes"
  vpc_id      = var.app_vpc_id

  ingress {
    description     = "Cluster communication"
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    security_groups = aws_security_group.eks_cluster.*.id
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-eks-nodes-sg"
  })
}

resource "aws_security_group_rule" "eks_cluster_from_nodes" {
  count                    = local.create_eks ? 1 : 0
  description              = "Allow worker nodes to reach the EKS control plane"
  type                     = "ingress"
  from_port                = 443
  to_port                  = 443
  protocol                 = "tcp"
  security_group_id        = aws_security_group.eks_cluster[0].id
  source_security_group_id = aws_security_group.eks_nodes[0].id
}

resource "aws_security_group_rule" "eks_nodes_self" {
  count                    = local.create_eks ? 1 : 0
  description              = "Allow node-to-node communication"
  type                     = "ingress"
  from_port                = 0
  to_port                  = 0
  protocol                 = "-1"
  security_group_id        = aws_security_group.eks_nodes[0].id
  source_security_group_id = aws_security_group.eks_nodes[0].id
}

resource "aws_iam_role" "eks_cluster" {
  count = local.create_eks ? 1 : 0

  name = "${local.name_prefix}-${var.region_key}-eks-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster" {
  count = local.create_eks ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_cluster[0].name
}

resource "aws_iam_role" "eks_node" {
  count = local.create_eks ? 1 : 0

  name = "${local.name_prefix}-${var.region_key}-eks-node-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_node_worker" {
  count = local.create_eks ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.eks_node[0].name
}

resource "aws_iam_role_policy_attachment" "eks_node_cni" {
  count = local.create_eks ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.eks_node[0].name
}

resource "aws_iam_role_policy_attachment" "eks_node_registry" {
  count = local.create_eks ? 1 : 0

  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.eks_node[0].name
}

resource "aws_eks_cluster" "this" {
  count = local.create_eks ? 1 : 0

  name     = coalesce(try(local.eks_config.name_prefix, null), format("${local.name_prefix}-%s", var.region_key))
  role_arn = aws_iam_role.eks_cluster[0].arn
  version  = local.eks_config.version

  vpc_config {
    subnet_ids         = var.app_subnet_ids
    security_group_ids = aws_security_group.eks_cluster.*.id
  }

  depends_on = [aws_iam_role_policy_attachment.eks_cluster]

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-eks"
  })
}

resource "aws_eks_node_group" "this" {
  count = local.create_eks ? 1 : 0

  cluster_name    = aws_eks_cluster.this[0].name
  node_group_name = "${local.name_prefix}-${var.region_key}-eks-default"
  node_role_arn   = aws_iam_role.eks_node[0].arn
  subnet_ids      = var.app_subnet_ids

  scaling_config {
    desired_size = local.eks_config.desired_capacity
    max_size     = local.eks_config.max_capacity
    min_size     = local.eks_config.min_capacity
  }

  ami_type       = "AL2_x86_64"
  disk_size      = 20
  instance_types = [local.eks_config.node_instance_type]
  remote_access {
    ec2_ssh_key = null
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_node_worker,
    aws_iam_role_policy_attachment.eks_node_cni,
    aws_iam_role_policy_attachment.eks_node_registry
  ]

  tags = merge(local.base_tags, {
    Name = "${local.name_prefix}-${var.region_key}-eks-node-group"
  })
}

# -------------------------------------------------------------
# Output metadata
# -------------------------------------------------------------
locals {
  metadata = {
    alb = local.create_alb ? {
      dns_name          = aws_lb.app[0].dns_name
      zone_id           = aws_lb.app[0].zone_id
      load_balancer_arn = aws_lb.app[0].arn
      target_group_arn  = aws_lb_target_group.app[0].arn
    } : null
    autoscaling = local.create_app_asg ? {
      asg_name           = aws_autoscaling_group.app[0].name
      launch_template_id = aws_launch_template.app[0].id
    } : null
    rds = local.create_rds ? {
      endpoint     = aws_db_instance.this[0].address
      db_name      = aws_db_instance.this[0].db_name
      username     = local.rds_config.username
      password_ssm = null
    } : null
    eks = local.create_eks ? {
      cluster_name = aws_eks_cluster.this[0].name
      node_group   = aws_eks_node_group.this[0].id
    } : null
    global_accelerator = local.create_ga ? {
      dns_name     = aws_globalaccelerator_accelerator.app[0].dns_name
      listener_arn = aws_globalaccelerator_listener.app[0].arn
    } : null
  }
}
