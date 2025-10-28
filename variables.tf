variable "aws_regions" {
  description = "AWS regional configuration for Skyforge."
  type = map(object({
    cidr_block              = string
    ipv6_prefix             = string
    availability_zones      = list(string)
    enable_transit_gateway  = bool
    enable_network_firewall = bool
    enable_gateway_lb       = bool
    gwlb_paloalto = optional(object({
      ami_id               = optional(string)
      instance_type        = optional(string, "m5.xlarge")
      instance_count       = optional(number, 2)
      iam_instance_profile = optional(string)
      user_data            = optional(string)
      name_prefix          = optional(string, "skyforge-palo")
      allowed_principals   = optional(list(string), [])
      bootstrap = optional(object({
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
      }))
    }))
    third_party_firewalls = optional(object({
      fortinet = optional(object({
        enable               = optional(bool, false)
        ami_id               = optional(string)
        instance_type        = optional(string, "c6i.large")
        iam_instance_profile = optional(string)
        user_data            = optional(string)
        name_prefix          = optional(string, "skyforge-fortinet")
      }))
      checkpoint = optional(object({
        enable               = optional(bool, false)
        ami_id               = optional(string)
        instance_type        = optional(string, "c6i.large")
        iam_instance_profile = optional(string)
        user_data            = optional(string)
        name_prefix          = optional(string, "skyforge-checkpoint")
      }))
    }))
    transit_gateway_connect = optional(object({
      enable        = optional(bool, false)
      transport_vpc = string
      subnet_tier   = string
      peer_address  = optional(string)
      peer_bgp_asn  = optional(number, 65001)
      inside_cidr   = optional(string, "169.254.100.0/29")
      protocol      = optional(string, "gre")
      connector = optional(object({
        enable               = optional(bool, true)
        ami_id               = optional(string)
        ami_product_code     = optional(string, "6njl1pau431dv1qxipg63mvah")
        instance_type        = optional(string, "c5.large")
        iam_instance_profile = optional(string)
        key_name             = optional(string)
        name_prefix          = optional(string, "skyforge-csr")
        advertised_prefixes  = optional(list(string), [])
        management_cidrs     = optional(list(string), ["10.0.0.0/8", "192.168.0.0/16"])
        user_data            = optional(string)
      }))
    }))
    app_stack = optional(object({
      enable                    = bool
      frontend_vpc              = string
      frontend_tier             = string
      app_vpc                   = string
      app_tier                  = string
      data_vpc                  = string
      data_tier                 = string
      create_eks                = optional(bool, true)
      create_alb                = optional(bool, true)
      create_rds                = optional(bool, true)
      create_app_asg            = optional(bool, true)
      create_global_accelerator = optional(bool, false)
      eks = optional(object({
        name_prefix        = optional(string)
        version            = optional(string, "1.29")
        node_instance_type = optional(string, "t3.medium")
        desired_capacity   = optional(number, 2)
        max_capacity       = optional(number, 3)
        min_capacity       = optional(number, 1)
      }))
      rds = optional(object({
        engine            = optional(string, "postgres")
        engine_version    = optional(string, "13.11")
        instance_class    = optional(string, "db.t3.micro")
        allocated_storage = optional(number, 20)
        database_name     = optional(string, "appdb")
        username          = optional(string, "appuser")
      }))
      asg = optional(object({
        desired_capacity = optional(number, 2)
        max_size         = optional(number, 3)
        min_size         = optional(number, 1)
        instance_type    = optional(string, "t3.micro")
      }))
    }))
    vpcs = map(object({
      cidr_block         = string
      tier_subnet_prefix = number
      tiers              = list(string)
    }))
    security_groups = optional(list(object({
      name        = string
      description = optional(string, "")
      vpc         = string
      ingress = optional(list(object({
        description      = optional(string)
        protocol         = string
        from_port        = number
        to_port          = number
        cidr_blocks      = optional(list(string))
        ipv6_cidr_blocks = optional(list(string))
      })), [])
      egress = optional(list(object({
        description      = optional(string)
        protocol         = string
        from_port        = number
        to_port          = number
        cidr_blocks      = optional(list(string))
        ipv6_cidr_blocks = optional(list(string))
      })), [])
    })), [])
    network_acls = optional(list(object({
      name         = string
      vpc          = string
      subnet_tiers = list(string)
      ingress = optional(list(object({
        rule_no   = number
        action    = string
        protocol  = string
        cidr      = optional(string)
        ipv6_cidr = optional(string)
        from_port = number
        to_port   = number
      })), [])
      egress = optional(list(object({
        rule_no   = number
        action    = string
        protocol  = string
        cidr      = optional(string)
        ipv6_cidr = optional(string)
        from_port = number
        to_port   = number
      })), [])
    })), [])
    network_firewall = optional(object({
      stateful_rules = optional(list(object({
        name             = string
        description      = optional(string, "")
        action           = string
        protocol         = string
        source           = string
        source_port      = optional(string, "any")
        destination      = string
        destination_port = optional(string, "any")
        sid              = number
        rev              = optional(number, 1)
      })), [])
      stateless_rules = optional(list(object({
        priority          = number
        action            = string
        source_cidrs      = optional(list(string), ["0.0.0.0/0"])
        destination_cidrs = optional(list(string), ["0.0.0.0/0"])
        source_ports = optional(list(object({
          from_port = number
          to_port   = number
        })), [])
        destination_ports = optional(list(object({
          from_port = number
          to_port   = number
        })), [])
        protocols = optional(list(string), ["tcp"])
      })), [])
      default_actions = optional(object({
        forward  = optional(list(string), ["aws:forward_to_sfe"])
        fragment = optional(list(string), ["aws:forward_to_sfe"])
      }), {})
    }))
  }))
  default = {
    "us-east-1" = {
      cidr_block              = "10.10.0.0/16"
      ipv6_prefix             = "2600:10::/56"
      availability_zones      = ["us-east-1a", "us-east-1b", "us-east-1c"]
      enable_transit_gateway  = true
      enable_network_firewall = false
      enable_gateway_lb       = true
      vpcs = {
        "shared-services" = {
          cidr_block         = "10.10.0.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "app", "data"]
        }
        "inspection" = {
          cidr_block         = "10.10.16.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "inspection", "egress"]
        }
        "eks-cluster" = {
          cidr_block         = "10.10.32.0/19"
          tier_subnet_prefix = 4
          tiers              = ["public", "private", "services"]
        }
        "dmz" = {
          cidr_block         = "10.10.64.0/20"
          tier_subnet_prefix = 4
          tiers              = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block         = "10.10.80.0/20"
          tier_subnet_prefix = 4
          tiers              = ["sandbox", "qa", "tools"]
        }
      }
    }
    "eu-central-1" = {
      cidr_block              = "10.20.0.0/16"
      ipv6_prefix             = "2600:20::/56"
      availability_zones      = ["eu-central-1a", "eu-central-1b", "eu-central-1c"]
      enable_transit_gateway  = true
      enable_network_firewall = true
      enable_gateway_lb       = false
      vpcs = {
        "shared-services" = {
          cidr_block         = "10.20.0.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "app", "data"]
        }
        "inspection" = {
          cidr_block         = "10.20.16.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "inspection", "egress"]
        }
        "eks-cluster" = {
          cidr_block         = "10.20.32.0/19"
          tier_subnet_prefix = 4
          tiers              = ["public", "private", "services"]
        }
        "dmz" = {
          cidr_block         = "10.20.64.0/20"
          tier_subnet_prefix = 4
          tiers              = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block         = "10.20.80.0/20"
          tier_subnet_prefix = 4
          tiers              = ["sandbox", "qa", "tools"]
        }
      }
    }
    "ap-northeast-1" = {
      cidr_block              = "10.30.0.0/16"
      ipv6_prefix             = "2600:30::/56"
      availability_zones      = ["ap-northeast-1a", "ap-northeast-1c", "ap-northeast-1d"]
      enable_transit_gateway  = true
      enable_network_firewall = false
      enable_gateway_lb       = false
      vpcs = {
        "shared-services" = {
          cidr_block         = "10.30.0.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "app", "data"]
        }
        "inspection" = {
          cidr_block         = "10.30.16.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "inspection", "egress"]
        }
        "eks-cluster" = {
          cidr_block         = "10.30.32.0/19"
          tier_subnet_prefix = 4
          tiers              = ["public", "private", "services"]
        }
        "dmz" = {
          cidr_block         = "10.30.64.0/20"
          tier_subnet_prefix = 4
          tiers              = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block         = "10.30.80.0/20"
          tier_subnet_prefix = 4
          tiers              = ["sandbox", "qa", "tools"]
        }
      }
    }
    "me-south-1" = {
      cidr_block              = "10.40.0.0/16"
      ipv6_prefix             = "2600:40::/56"
      availability_zones      = ["me-south-1a", "me-south-1b", "me-south-1c"]
      enable_transit_gateway  = true
      enable_network_firewall = false
      enable_gateway_lb       = false
      vpcs = {
        "shared-services" = {
          cidr_block         = "10.40.0.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "app", "data"]
        }
        "inspection" = {
          cidr_block         = "10.40.16.0/20"
          tier_subnet_prefix = 4
          tiers              = ["ingress", "inspection", "egress"]
        }
        "eks-cluster" = {
          cidr_block         = "10.40.32.0/19"
          tier_subnet_prefix = 4
          tiers              = ["public", "private", "services"]
        }
        "dmz" = {
          cidr_block         = "10.40.64.0/20"
          tier_subnet_prefix = 4
          tiers              = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block         = "10.40.80.0/20"
          tier_subnet_prefix = 4
          tiers              = ["sandbox", "qa", "tools"]
        }
      }
    }
  }
}

variable "aws_tgw_mesh_links" {
  description = "Optional list of AWS Transit Gateway mesh links. Leave empty to automatically mesh every region with a TGW."
  type = list(object({
    source_region = string
    target_region = string
  }))
  default = []
}

variable "azure_regions" {
  description = "Azure regional configuration for Skyforge."
  type = map(object({
    cidr_block                 = string
    ipv6_prefix                = string
    location                   = string
    enable_virtual_wan         = bool
    enable_firewall            = bool
    virtual_hub_address_prefix = optional(string)
    nat_gateways = optional(list(object({
      name                 = string
      vnet_name            = string
      public_subnet_tier   = string
      private_subnet_tiers = list(string)
    })), [])
    asa_nva = optional(object({
      enable         = optional(bool, true)
      vnet_name      = string
      subnet_tier    = string
      vm_size        = optional(string)
      admin_username = optional(string)
      admin_password = optional(string)
    }), null)
    vnet_peerings = optional(list(object({
      name   = string
      vnet_a = string
      vnet_b = string
    })), [])
    private_endpoints = optional(list(object({
      name                 = string
      vnet_name            = string
      subnet_tier          = string
      resource_id          = string
      is_manual_connection = optional(bool, false)
      subresource_names    = optional(list(string))
      request_message      = optional(string)
      private_dns_zone_ids = optional(list(string), [])
    })), [])
    vnets = map(object({
      address_space      = list(string)
      subnets_per_tier   = number
      tiers              = list(string)
      enable_private_dns = bool
    }))
    nva_chain = optional(object({
      enable = optional(bool, false)
      route_collections = optional(map(object({
        tiers                   = list(string)
        destinations            = optional(list(string), ["0.0.0.0/0"])
        next_hop_type           = optional(string, "VirtualAppliance")
        next_hop_ip             = optional(string)
        use_firewall_private_ip = optional(bool, false)
        disable_bgp_propagation = optional(bool, false)
      })), {})
    }), null)
    workloads = optional(object({
      aks = optional(object({
        vnet_name          = string
        subnet_tier        = string
        node_count         = optional(number, 2)
        node_vm_size       = optional(string, "Standard_DS2_v2")
        kubernetes_version = optional(string)
      }))
      app_gateway = optional(object({
        vnet_name   = string
        subnet_tier = string
        sku_name    = optional(string, "WAF_v2")
        capacity    = optional(number, 2)
      }))
      app_service = optional(object({
        linux_runtime = optional(bool, true)
        sku_name      = optional(string, "P1v3")
      }))
      storage = optional(object({
        sku  = optional(string, "Standard_LRS")
        kind = optional(string, "StorageV2")
      }))
      sql = optional(object({
        administrator_login    = optional(string, "skyforgeadmin")
        administrator_password = optional(string)
        sku_name               = optional(string, "GP_Gen5_2")
        max_size_gb            = optional(number, 32)
      }))
      load_balancer = optional(object({
        sku  = optional(string, "Standard")
        type = optional(string, "Public")
        frontend = optional(object({
          vnet_name          = string
          subnet_tier        = string
          private_ip_address = optional(string)
        }))
        backend_addresses = optional(list(object({
          name        = optional(string)
          ip_address  = string
          vnet_name   = string
          subnet_tier = string
        })), [])
        health_probe = optional(object({
          name                = optional(string)
          protocol            = optional(string, "Tcp")
          port                = optional(number)
          path                = optional(string)
          interval_in_seconds = optional(number, 5)
          number_of_probes    = optional(number, 2)
        }))
        rules = optional(list(object({
          name                  = string
          protocol              = string
          frontend_port         = number
          backend_port          = number
          idle_timeout          = optional(number, 4)
          enable_floating_ip    = optional(bool, false)
          load_distribution     = optional(string, "Default")
          disable_outbound_snat = optional(bool, false)
        })), [])
      }))
      front_door = optional(object({
        profile_name             = optional(string)
        endpoint_name            = optional(string)
        origin_host              = string
        origin_host_header       = optional(string)
        http_port                = optional(number, 80)
        https_port               = optional(number, 443)
        health_probe_path        = optional(string, "/")
        forwarding_protocol      = optional(string, "HttpsOnly")
        supported_protocols      = optional(list(string), ["Http", "Https"])
        patterns_to_match        = optional(list(string), ["/*"])
        sku_name                 = optional(string, "Standard_AzureFrontDoor")
        session_affinity_enabled = optional(bool, false)
        https_redirect_enabled   = optional(bool, true)
      }))
    }), null)
  }))
  default = {
    "uswest2" = {
      cidr_block         = "10.50.0.0/16"
      ipv6_prefix        = "2600:50::/56"
      location           = "westus2"
      enable_virtual_wan = true
      enable_firewall    = true
      vnets = {
        "shared-services" = {
          address_space      = ["10.50.0.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "app", "data"]
          enable_private_dns = true
        }
        "inspection" = {
          address_space      = ["10.50.16.0/20"]
          subnets_per_tier   = 2
          tiers              = ["ingress", "inspection", "egress"]
          enable_private_dns = false
        }
        "aks" = {
          address_space      = ["10.50.32.0/19"]
          subnets_per_tier   = 2
          tiers              = ["nodepool", "services", "ingress"]
          enable_private_dns = true
        }
        "dmz" = {
          address_space      = ["10.50.64.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "mid", "backend"]
          enable_private_dns = false
        }
        "app-services" = {
          address_space      = ["10.50.80.0/20"]
          subnets_per_tier   = 2
          tiers              = ["apps", "integration", "data"]
          enable_private_dns = true
        }
      }
    }
    "ireland" = {
      cidr_block         = "10.60.0.0/16"
      ipv6_prefix        = "2600:60::/56"
      location           = "northeurope"
      enable_virtual_wan = true
      enable_firewall    = true
      vnets = {
        "shared-services" = {
          address_space      = ["10.60.0.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "app", "data"]
          enable_private_dns = true
        }
        "inspection" = {
          address_space      = ["10.60.16.0/20"]
          subnets_per_tier   = 2
          tiers              = ["ingress", "inspection", "egress"]
          enable_private_dns = false
        }
        "aks" = {
          address_space      = ["10.60.32.0/19"]
          subnets_per_tier   = 2
          tiers              = ["nodepool", "services", "ingress"]
          enable_private_dns = true
        }
        "dmz" = {
          address_space      = ["10.60.64.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "mid", "backend"]
          enable_private_dns = false
        }
        "app-services" = {
          address_space      = ["10.60.80.0/20"]
          subnets_per_tier   = 2
          tiers              = ["apps", "integration", "data"]
          enable_private_dns = true
        }
      }
    }
    "tokyo" = {
      cidr_block         = "10.70.0.0/16"
      ipv6_prefix        = "2600:70::/56"
      location           = "japaneast"
      enable_virtual_wan = true
      enable_firewall    = true
      vnets = {
        "shared-services" = {
          address_space      = ["10.70.0.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "app", "data"]
          enable_private_dns = true
        }
        "inspection" = {
          address_space      = ["10.70.16.0/20"]
          subnets_per_tier   = 2
          tiers              = ["ingress", "inspection", "egress"]
          enable_private_dns = false
        }
        "aks" = {
          address_space      = ["10.70.32.0/19"]
          subnets_per_tier   = 2
          tiers              = ["nodepool", "services", "ingress"]
          enable_private_dns = true
        }
        "dmz" = {
          address_space      = ["10.70.64.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "mid", "backend"]
          enable_private_dns = false
        }
        "app-services" = {
          address_space      = ["10.70.80.0/20"]
          subnets_per_tier   = 2
          tiers              = ["apps", "integration", "data"]
          enable_private_dns = true
        }
      }
    }
    "southafrica" = {
      cidr_block         = "10.80.0.0/16"
      ipv6_prefix        = "2600:80::/56"
      location           = "southafricanorth"
      enable_virtual_wan = true
      enable_firewall    = true
      vnets = {
        "shared-services" = {
          address_space      = ["10.80.0.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "app", "data"]
          enable_private_dns = true
        }
        "inspection" = {
          address_space      = ["10.80.16.0/20"]
          subnets_per_tier   = 2
          tiers              = ["ingress", "inspection", "egress"]
          enable_private_dns = false
        }
        "aks" = {
          address_space      = ["10.80.32.0/19"]
          subnets_per_tier   = 2
          tiers              = ["nodepool", "services", "ingress"]
          enable_private_dns = true
        }
        "dmz" = {
          address_space      = ["10.80.64.0/20"]
          subnets_per_tier   = 2
          tiers              = ["frontend", "mid", "backend"]
          enable_private_dns = false
        }
        "app-services" = {
          address_space      = ["10.80.80.0/20"]
          subnets_per_tier   = 2
          tiers              = ["apps", "integration", "data"]
          enable_private_dns = true
        }
      }
    }
  }
}

variable "azure_resource_group_name" {
  description = "Optional override to deploy all Azure resources into a pre-created resource group (deprecated, prefer azure_global_resource_group)."
  type        = string
  default     = null
}

variable "azure_global_resource_group" {
  description = "Configuration for a shared Azure resource group reused across regions."
  type = object({
    name              = string
    location          = optional(string)
    create_if_missing = optional(bool, false)
    tags              = optional(map(string), {})
  })
  default = null
}

variable "gcp_regions" {
  description = "GCP regional configuration for Skyforge."
  type = map(object({
    project_id        = optional(string)
    region            = string
    cidr_block        = string
    ipv6_prefix       = string
    enable_ha_vpn     = bool
    enable_cloud_natt = bool
    vpcs = map(object({
      cidr_block              = string
      routing_mode            = string
      subnet_count            = number
      create_firewall         = bool
      subnet_prefix_extension = number
      tier_labels             = list(string)
    }))
    workloads = optional(object({
      gke = optional(object({
        vpc_name        = string
        subnet_tier     = string
        node_count      = optional(number, 2)
        node_machine    = optional(string, "e2-standard-2")
        release_channel = optional(string, "REGULAR")
      }))
      cloud_armor = optional(object({
        action = optional(string, "ALLOW")
      }))
      storage = optional(object({
        location = optional(string, "US")
      }))
      cloud_run = optional(object({
        service_name          = optional(string)
        image                 = string
        port                  = optional(number, 8080)
        allow_unauthenticated = optional(bool, true)
      }))
      cloud_function = optional(object({
        name                  = string
        runtime               = optional(string, "python310")
        entry_point           = optional(string, "function")
        source_archive_bucket = string
        source_archive_object = string
        trigger_http          = optional(bool, true)
      }))
      pubsub = optional(object({
        topic_name        = string
        subscription_name = string
      }))
      sql = optional(object({
        database_version = optional(string, "POSTGRES_15")
        tier             = optional(string, "db-custom-2-7680")
        region           = optional(string)
      }))
      global_lb = optional(object({
        backend_type      = optional(string, "cloud_run")
        forwarding_scheme = optional(string, "HTTP")
        enable_cdn        = optional(bool, false)
        description       = optional(string)
      }))
    }), null)
  }))
  default = {
    "us-central1" = {
      project_id        = "your-gcp-project-id"
      region            = "us-central1"
      cidr_block        = "10.90.0.0/16"
      ipv6_prefix       = "2600:90::/56"
      enable_ha_vpn     = true
      enable_cloud_natt = true
      vpcs = {
        "shared-services" = {
          cidr_block              = "10.90.0.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "app", "data"]
        }
        "inspection" = {
          cidr_block              = "10.90.16.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["ingress", "inspection", "egress"]
        }
        "gke" = {
          cidr_block              = "10.90.32.0/19"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["nodes", "services", "ingress"]
        }
        "dmz" = {
          cidr_block              = "10.90.64.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block              = "10.90.80.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = false
          subnet_prefix_extension = 4
          tier_labels             = ["sandbox", "qa", "tools"]
        }
      }
    }
    "europe-west1" = {
      project_id        = "your-gcp-project-id"
      region            = "europe-west1"
      cidr_block        = "10.100.0.0/16"
      ipv6_prefix       = "2600:100::/56"
      enable_ha_vpn     = true
      enable_cloud_natt = true
      vpcs = {
        "shared-services" = {
          cidr_block              = "10.100.0.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "app", "data"]
        }
        "inspection" = {
          cidr_block              = "10.100.16.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["ingress", "inspection", "egress"]
        }
        "gke" = {
          cidr_block              = "10.100.32.0/19"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["nodes", "services", "ingress"]
        }
        "dmz" = {
          cidr_block              = "10.100.64.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block              = "10.100.80.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = false
          subnet_prefix_extension = 4
          tier_labels             = ["sandbox", "qa", "tools"]
        }
      }
    }
    "asia-southeast1" = {
      project_id        = "your-gcp-project-id"
      region            = "asia-southeast1"
      cidr_block        = "10.110.0.0/16"
      ipv6_prefix       = "2600:110::/56"
      enable_ha_vpn     = true
      enable_cloud_natt = true
      vpcs = {
        "shared-services" = {
          cidr_block              = "10.110.0.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "app", "data"]
        }
        "inspection" = {
          cidr_block              = "10.110.16.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["ingress", "inspection", "egress"]
        }
        "gke" = {
          cidr_block              = "10.110.32.0/19"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["nodes", "services", "ingress"]
        }
        "dmz" = {
          cidr_block              = "10.110.64.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block              = "10.110.80.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = false
          subnet_prefix_extension = 4
          tier_labels             = ["sandbox", "qa", "tools"]
        }
      }
    }
    "me-west1" = {
      project_id        = "your-other-project-id"
      region            = "me-west1"
      cidr_block        = "10.120.0.0/16"
      ipv6_prefix       = "2600:120::/56"
      enable_ha_vpn     = true
      enable_cloud_natt = true
      vpcs = {
        "shared-services" = {
          cidr_block              = "10.120.0.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "app", "data"]
        }
        "inspection" = {
          cidr_block              = "10.120.16.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["ingress", "inspection", "egress"]
        }
        "gke" = {
          cidr_block              = "10.120.32.0/19"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["nodes", "services", "ingress"]
        }
        "dmz" = {
          cidr_block              = "10.120.64.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = true
          subnet_prefix_extension = 4
          tier_labels             = ["frontend", "mid", "backend"]
        }
        "lab" = {
          cidr_block              = "10.120.80.0/20"
          routing_mode            = "GLOBAL"
          subnet_count            = 3
          create_firewall         = false
          subnet_prefix_extension = 4
          tier_labels             = ["sandbox", "qa", "tools"]
        }
      }
    }
  }
}

variable "vnf_endpoints" {
  description = "Virtual network function endpoints for VPN termination."
  type = map(object({
    location         = string
    ipv4_cidr        = string
    ipv6_prefix      = string
    device_type      = string
    tunnel_count     = number
    connect_to_cloud = optional(bool, true)
    vpn_gateway_ipv6 = optional(string)
    vpn_gateway_ipv4 = optional(string)
  }))
  default = {
    "san_jose" = {
      location         = "San Jose, CA"
      ipv4_cidr        = "172.16.0.0/16"
      ipv6_prefix      = "fd00:16::/48"
      device_type      = "palo_alto_vm"
      tunnel_count     = 8
      connect_to_cloud = true
    }
    "atlanta" = {
      location         = "Atlanta, GA"
      ipv4_cidr        = "172.17.0.0/16"
      ipv6_prefix      = "fd00:17::/48"
      device_type      = "palo_alto_vm"
      tunnel_count     = 8
      connect_to_cloud = true
    }
  }
}

variable "vpn_mesh" {
  description = "Declarative definition of mesh VPN connections between clouds and VNF endpoints."
  type = object({
    cloud_links = list(object({
      source = object({
        cloud  = string
        region = string
        hub_id = string
      })
      target = object({
        cloud  = string
        region = string
        hub_id = string
      })
      bgp = object({
        source_asn = number
        target_asn = number
      })
      tunnels = list(object({
        id              = string
        preferred_proto = string
        fallback_proto  = optional(string)
        source_endpoint = string
        target_endpoint = string
      }))
    }))
    vnf_links = list(object({
      site                  = string
      cloud                 = string
      region                = string
      hub_id                = string
      bgp_asn               = number
      tunnel_mode           = string
      preferred_proto       = string
      customer_gateway_ipv6 = optional(string)
      customer_gateway_ipv4 = optional(string)
    }))
  })
  default = {
    cloud_links = []
    vnf_links   = []
  }
}

variable "aws_global_app" {
  description = "Settings for the AWS Global Accelerator that fronts the multi-region application."
  type = object({
    enable         = optional(bool, true)
    listener_ports = optional(list(number), [80, 443])
    health_check = optional(object({
      protocol  = optional(string)
      port      = optional(number)
      interval  = optional(number)
      threshold = optional(number)
    }), {})
    endpoint_weights = optional(map(number), {})
    record_name      = optional(string, "global-app")
  })
  nullable = false
  default  = {}
}

variable "aws_network_manager" {
  description = "Configuration for AWS Network Manager global network and sites."
  type = object({
    enable      = optional(bool, false)
    global_name = optional(string, "skyforge-global-network")
    description = optional(string)
    sites = optional(list(object({
      name        = string
      description = optional(string)
      region      = optional(string)
      address     = optional(string)
      latitude    = optional(number)
      longitude   = optional(number)
    })), [])
    devices = optional(list(object({
      name        = string
      site_name   = string
      type        = optional(string)
      description = optional(string)
      model       = optional(string)
      serial      = optional(string)
    })), [])
  })
  default = null
}

variable "aws_reachability" {
  description = "Per-region AWS Reachability Analyzer path definitions keyed by region code."
  type = map(object({
    paths = list(object({
      name             = string
      description      = optional(string)
      source_vpc       = string
      destination_vpc  = string
      protocol         = optional(string, "TCP")
      source_port      = optional(number)
      destination_port = optional(number)
      perform_analysis = optional(bool, true)
    }))
  }))
  default = {}
}

variable "azure_reachability" {
  description = "Per-region Azure Network Watcher connection monitor definitions."
  type = map(object({
    tests = optional(list(object({
      name                = string
      source_address      = string
      destination_address = string
      protocol            = optional(string, "Tcp")
      destination_port    = optional(number, 443)
      frequency_seconds   = optional(number, 60)
      enabled             = optional(bool, true)
      description         = optional(string)
      trace_route_enabled = optional(bool, true)
    })), [])
  }))
  default = {}
}

variable "gcp_reachability" {
  description = "Per-region GCP Network Management connectivity tests."
  type = map(object({
    project_id = optional(string)
    tests = optional(list(object({
      name             = string
      source_ip        = string
      destination_ip   = string
      protocol         = optional(string, "TCP")
      destination_port = optional(number, 443)
      source_port      = optional(number)
      description      = optional(string)
      related_projects = optional(list(string), [])
      enabled          = optional(bool, true)
    })), [])
  }))
  default = {}
}

variable "dns" {
  description = "Optional global DNS zone configuration."
  type = object({
    enable       = bool
    domain       = string
    comment      = optional(string, "Skyforge DNS zone")
    private_zone = optional(bool, false)
    tags         = optional(map(string), {})
    vpc_associations = optional(list(object({
      vpc_id = string
      region = string
    })), [])
    records = optional(map(object({
      name   = optional(string)
      type   = string
      ttl    = optional(number, 300)
      values = optional(list(string), [])
      alias = optional(object({
        name                   = string
        zone_id                = string
        evaluate_target_health = optional(bool, false)
      }))
    })), {})
  })
  default = null
}

variable "default_tags" {
  description = "Default tag map applied to supported resources."
  type        = map(string)
  default = {
    Project     = "skyforge"
    Environment = "demo"
    Skyforge    = "true"
  }
}

variable "deployment_identifier" {
  description = "Optional identifier (e.g., username, ticket) appended to resource names and tags. When omitted, Skyforge falls back to a timestamp."
  type        = string
  default     = ""
}
