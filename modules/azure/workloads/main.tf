locals {
  cfg                = try(var.config, {})
  front_door_cfg_raw = try(local.cfg.front_door, null)
  aks_cfg            = try(local.cfg.aks, null)
  app_gw_cfg         = try(local.cfg.app_gateway, null)
  app_gw_sku_name    = coalesce(try(local.app_gw_cfg.sku_name, null), "Standard_v2")
  app_gw_sku_tier    = coalesce(try(local.app_gw_cfg.sku_tier, null), local.app_gw_sku_name)
  app_service_cfg    = try(local.cfg.app_service, null)
  storage_cfg        = try(local.cfg.storage, null)
  sql_cfg            = try(local.cfg.sql, null)
  load_balancer_cfg  = try(local.cfg.load_balancer, null)
  front_door_cfg = (
    local.front_door_cfg_raw != null &&
    trimspace(try(local.front_door_cfg_raw.origin_host, "")) != ""
  ) ? local.front_door_cfg_raw : null

  aks_enabled           = local.aks_cfg != null
  app_gw_enabled        = local.app_gw_cfg != null
  app_service_enabled   = local.app_service_cfg != null
  storage_enabled       = local.storage_cfg != null
  sql_enabled           = local.sql_cfg != null
  front_door_enabled    = local.front_door_cfg != null
  load_balancer_enabled = local.load_balancer_cfg != null

  load_balancer_sku       = coalesce(try(local.load_balancer_cfg.sku, null), "Standard")
  load_balancer_type      = lower(coalesce(try(local.load_balancer_cfg.type, null), "public"))
  load_balancer_is_public = local.load_balancer_type == "public"
  load_balancer_frontend  = local.load_balancer_enabled ? try(local.load_balancer_cfg.frontend, null) : null
  load_balancer_frontend_key = (
    local.load_balancer_enabled && !local.load_balancer_is_public
    ) ? format(
    "%s.%s",
    try(local.load_balancer_frontend.vnet_name, ""),
    try(local.load_balancer_frontend.subnet_tier, "")
  ) : null
  load_balancer_frontend_subnet_id = (
    local.load_balancer_enabled &&
    !local.load_balancer_is_public &&
    local.load_balancer_frontend_key != null
  ) ? lookup(var.subnet_id_map, local.load_balancer_frontend_key, null) : null
  load_balancer_frontend_name = local.load_balancer_is_public ? "public-frontend" : "internal-frontend"

  load_balancer_backend_addresses_raw = local.load_balancer_enabled ? try(local.load_balancer_cfg.backend_addresses, []) : []
  load_balancer_backend_addresses = [
    for addr in local.load_balancer_backend_addresses_raw : merge(addr, {
      key                = coalesce(try(addr.name, null), replace(addr.ip_address, ".", "-"))
      virtual_network_id = lookup(var.vnet_id_map, addr.vnet_name, null)
    })
  ]
  load_balancer_backend_address_map = {
    for addr in local.load_balancer_backend_addresses : addr.key => addr
  }

  load_balancer_rules_input = local.load_balancer_enabled ? try(local.load_balancer_cfg.rules, []) : []
  load_balancer_rules_fallback = [
    {
      name          = "tcp-80"
      protocol      = "Tcp"
      frontend_port = 80
      backend_port  = 80
      idle_timeout  = 4
    }
  ]
  load_balancer_rules = local.load_balancer_enabled ? (
    length(local.load_balancer_rules_input) > 0 ? local.load_balancer_rules_input : local.load_balancer_rules_fallback
  ) : []
  load_balancer_rule_map = {
    for rule in local.load_balancer_rules :
    rule.name => merge(rule, {
      protocol          = title(lower(rule.protocol))
      idle_timeout      = try(rule.idle_timeout, 4)
      enable_floating   = try(rule.enable_floating_ip, false)
      load_distribution = coalesce(try(rule.load_distribution, null), "Default")
      disable_snat      = try(rule.disable_outbound_snat, false)
    })
  }

  load_balancer_probe_cfg = local.load_balancer_enabled ? (
    try(local.load_balancer_cfg.health_probe, null) != null
    ? local.load_balancer_cfg.health_probe
    : {
      name     = "tcp-backend"
      protocol = "Tcp"
      port     = local.load_balancer_rules[0].backend_port
    }
  ) : null
  load_balancer_probe_protocol = title(lower(coalesce(try(local.load_balancer_probe_cfg.protocol, null), "Tcp")))
  load_balancer_probe_port     = try(local.load_balancer_probe_cfg.port, local.load_balancer_rules != [] ? local.load_balancer_rules[0].backend_port : 80)
  load_balancer_probe_path     = local.load_balancer_probe_protocol == "Http" ? coalesce(try(local.load_balancer_probe_cfg.path, null), "/") : null
  load_balancer_probe_interval = try(local.load_balancer_probe_cfg.interval_in_seconds, 5)
  load_balancer_probe_count    = try(local.load_balancer_probe_cfg.number_of_probes, 2)

  base_tags = merge(var.default_tags, {
    SkyforgeRegion = var.region_key
    SkyforgeRole   = "application"
  })
  name_prefix         = var.resource_suffix == "" ? "skyforge" : "skyforge-${var.resource_suffix}"
  name_prefix_compact = var.resource_suffix == "" ? "skyforge" : format("skyforge%s", var.resource_suffix)

  aks_subnet_id = local.aks_enabled ? lookup(
    var.subnet_id_map,
    "${local.aks_cfg.vnet_name}.${local.aks_cfg.subnet_tier}",
    null
  ) : null

  app_gw_subnet_id = local.app_gw_enabled ? lookup(
    var.subnet_id_map,
    "${local.app_gw_cfg.vnet_name}.${local.app_gw_cfg.subnet_tier}",
    null
  ) : null
}

resource "random_string" "storage" {
  count = local.storage_enabled ? 1 : 0

  length  = 6
  special = false
  upper   = false
}

resource "azurerm_storage_account" "this" {
  count = local.storage_enabled ? 1 : 0

  name                     = lower(substr(format("%s%s%s", local.name_prefix_compact, var.region_key, random_string.storage[0].result), 0, 24))
  resource_group_name      = var.resource_group_name
  location                 = var.location
  account_tier             = split("_", try(local.storage_cfg.sku, "Standard_LRS"))[0]
  account_replication_type = split("_", try(local.storage_cfg.sku, "Standard_LRS"))[1]
  account_kind             = try(local.storage_cfg.kind, "StorageV2")

  tags = merge(local.base_tags, {
    Name = format("%s-%s-storage", local.name_prefix, var.region_key)
  })
}

resource "azurerm_kubernetes_cluster" "this" {
  count = local.aks_enabled ? 1 : 0

  name                = format("%s-%s-aks", local.name_prefix, var.region_key)
  location            = var.location
  resource_group_name = var.resource_group_name
  dns_prefix          = format("%s-%s", local.name_prefix, var.region_key)
  kubernetes_version  = try(local.aks_cfg.kubernetes_version, null)

  default_node_pool {
    name                 = "nodepool"
    vm_size              = try(local.aks_cfg.node_vm_size, "Standard_DS2_v2")
    node_count           = try(local.aks_cfg.node_count, 2)
    vnet_subnet_id       = local.aks_subnet_id
    orchestrator_version = try(local.aks_cfg.kubernetes_version, null)
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
  }

  lifecycle {
    precondition {
      condition     = local.aks_subnet_id != null
      error_message = "AKS subnet lookup failed. Ensure vnet_name and subnet_tier are valid."
    }
  }

  tags = merge(local.base_tags, {
    Name = format("%s-%s-aks", local.name_prefix, var.region_key)
  })
}

resource "azurerm_public_ip" "app_gateway" {
  count = local.app_gw_enabled ? 1 : 0

  name                = format("pip-%s-%s-appgw", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location
  allocation_method   = "Static"
  sku                 = "Standard"

  tags = merge(local.base_tags, {
    Name = format("pip-%s-%s-appgw", local.name_prefix, var.region_key)
  })
}

resource "azurerm_application_gateway" "this" {
  count = local.app_gw_enabled ? 1 : 0

  name                = format("agw-%s-%s", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location

  sku {
    name     = local.app_gw_sku_name
    tier     = local.app_gw_sku_tier
    capacity = try(local.app_gw_cfg.capacity, 2)
  }

  firewall_policy_id = try(local.app_gw_cfg.firewall_policy_id, null)

  gateway_ip_configuration {
    name      = "gateway-ipcfg"
    subnet_id = local.app_gw_subnet_id
  }

  frontend_port {
    name = "frontend-port"
    port = 80
  }

  frontend_ip_configuration {
    name                 = "public-frontend"
    public_ip_address_id = azurerm_public_ip.app_gateway[0].id
  }

  backend_address_pool {
    name = "default-backend"
  }

  backend_http_settings {
    name                  = "default-http"
    cookie_based_affinity = "Disabled"
    port                  = 80
    protocol              = "Http"
    request_timeout       = 30
  }

  http_listener {
    name                           = "http-listener"
    frontend_ip_configuration_name = "public-frontend"
    frontend_port_name             = "frontend-port"
    protocol                       = "Http"
  }

  request_routing_rule {
    name                       = "rule-1"
    rule_type                  = "Basic"
    http_listener_name         = "http-listener"
    backend_address_pool_name  = "default-backend"
    backend_http_settings_name = "default-http"
    priority                   = try(local.app_gw_cfg.priority, 100)
  }

  lifecycle {
    precondition {
      condition     = local.app_gw_subnet_id != null
      error_message = "Application Gateway subnet lookup failed. Ensure vnet_name and subnet_tier are valid."
    }

    precondition {
      condition = !(
        can(regex("(?i)WAF", local.app_gw_sku_name)) &&
        try(local.app_gw_cfg.firewall_policy_id, null) == null
      )
      error_message = "Application Gateway WAF SKU requires workloads.app_gateway.firewall_policy_id to reference a WAF policy."
    }
  }

  tags = merge(local.base_tags, {
    Name = format("agw-%s-%s", local.name_prefix, var.region_key)
  })
}

resource "azurerm_public_ip" "load_balancer" {
  count = local.load_balancer_enabled && local.load_balancer_is_public ? 1 : 0

  name                = format("pip-%s-%s-lb", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location
  allocation_method   = "Static"
  sku                 = local.load_balancer_sku

  tags = merge(local.base_tags, {
    Name         = format("pip-%s-%s-lb", local.name_prefix, var.region_key)
    SkyforgeRole = "load-balancer"
  })
}

resource "azurerm_lb" "standard" {
  count               = local.load_balancer_enabled ? 1 : 0
  name                = format("lb-%s-%s", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location
  sku                 = local.load_balancer_sku

  dynamic "frontend_ip_configuration" {
    for_each = local.load_balancer_is_public ? [1] : []
    content {
      name                 = local.load_balancer_frontend_name
      public_ip_address_id = azurerm_public_ip.load_balancer[0].id
    }
  }

  dynamic "frontend_ip_configuration" {
    for_each = local.load_balancer_is_public ? [] : [1]
    content {
      name                          = local.load_balancer_frontend_name
      subnet_id                     = local.load_balancer_frontend_subnet_id
      private_ip_address_allocation = try(local.load_balancer_frontend.private_ip_address, null) != null ? "Static" : "Dynamic"
      private_ip_address            = try(local.load_balancer_frontend.private_ip_address, null)
    }
  }

  tags = merge(local.base_tags, {
    Name = format("lb-%s-%s", local.name_prefix, var.region_key)
  })

  lifecycle {
    precondition {
      condition     = local.load_balancer_is_public || local.load_balancer_frontend_subnet_id != null
      error_message = "Internal load balancer requires load_balancer.frontend.vnet_name and subnet_tier to resolve a subnet ID."
    }
  }
}

resource "azurerm_lb_backend_address_pool" "standard" {
  count           = local.load_balancer_enabled ? 1 : 0
  loadbalancer_id = azurerm_lb.standard[0].id
  name            = "backend-pool"
}

resource "azurerm_lb_backend_address_pool_address" "standard" {
  for_each = local.load_balancer_enabled ? local.load_balancer_backend_address_map : {}

  name                    = each.key
  backend_address_pool_id = azurerm_lb_backend_address_pool.standard[0].id
  ip_address              = each.value.ip_address
  virtual_network_id      = each.value.virtual_network_id

  lifecycle {
    precondition {
      condition     = each.value.virtual_network_id != null
      error_message = "Load balancer backend address references an unknown VNet."
    }
  }
}

resource "azurerm_lb_probe" "standard" {
  count = local.load_balancer_enabled ? 1 : 0

  loadbalancer_id     = azurerm_lb.standard[0].id
  name                = coalesce(try(local.load_balancer_probe_cfg.name, null), "tcp-backend")
  protocol            = local.load_balancer_probe_protocol
  port                = local.load_balancer_probe_port
  interval_in_seconds = local.load_balancer_probe_interval
  number_of_probes    = local.load_balancer_probe_count
  request_path        = local.load_balancer_probe_path

  lifecycle {
    precondition {
      condition     = !(local.load_balancer_probe_protocol == "Tcp" && local.load_balancer_probe_path != null)
      error_message = "TCP probes must not define a request path."
    }
  }
}

resource "azurerm_lb_rule" "standard" {
  for_each = local.load_balancer_enabled ? local.load_balancer_rule_map : {}

  loadbalancer_id                = azurerm_lb.standard[0].id
  name                           = each.key
  protocol                       = each.value.protocol
  frontend_ip_configuration_name = local.load_balancer_frontend_name
  frontend_port                  = each.value.frontend_port
  backend_port                   = each.value.backend_port
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.standard[0].id]
  idle_timeout_in_minutes        = each.value.idle_timeout
  floating_ip_enabled            = each.value.enable_floating
  load_distribution              = each.value.load_distribution
  disable_outbound_snat          = each.value.disable_snat
  probe_id                       = azurerm_lb_probe.standard[0].id

  depends_on = [azurerm_lb_backend_address_pool_address.standard]
}

resource "azurerm_service_plan" "this" {
  count = local.app_service_enabled ? 1 : 0

  name                = format("asp-%s-%s", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location
  os_type             = try(local.app_service_cfg.linux_runtime, true) ? "Linux" : "Windows"
  sku_name            = try(local.app_service_cfg.sku_name, "P1v3")

  tags = merge(local.base_tags, {
    Name = format("asp-%s-%s", local.name_prefix, var.region_key)
  })
}

resource "azurerm_linux_web_app" "this" {
  count = local.app_service_enabled && try(local.app_service_cfg.linux_runtime, true) ? 1 : 0

  name                = format("app-%s-%s", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location
  service_plan_id     = azurerm_service_plan.this[0].id

  site_config {
    always_on = true

    application_stack {
      python_version = "3.11"
    }
  }

  tags = merge(local.base_tags, {
    Name = format("app-%s-%s", local.name_prefix, var.region_key)
  })
}

resource "azurerm_windows_web_app" "this" {
  count = local.app_service_enabled && !try(local.app_service_cfg.linux_runtime, true) ? 1 : 0

  name                = format("app-%s-%s", local.name_prefix, var.region_key)
  resource_group_name = var.resource_group_name
  location            = var.location
  service_plan_id     = azurerm_service_plan.this[0].id

  site_config {
    always_on = true
  }

  tags = merge(local.base_tags, {
    Name = format("app-%s-%s", local.name_prefix, var.region_key)
  })
}

resource "random_password" "sql" {
  count = local.sql_enabled && try(local.sql_cfg.administrator_password, null) == null ? 1 : 0

  length  = 24
  special = true
}

locals {
  sql_admin_password = local.sql_enabled ? coalesce(
    try(local.sql_cfg.administrator_password, null),
    try(random_password.sql[0].result, null)
  ) : null
}

resource "azurerm_mssql_server" "this" {
  count = local.sql_enabled ? 1 : 0

  name                         = lower(format("sql-%s-%s", local.name_prefix_compact, var.region_key))
  resource_group_name          = var.resource_group_name
  location                     = var.location
  version                      = "12.0"
  administrator_login          = try(local.sql_cfg.administrator_login, "skyforgeadmin")
  administrator_login_password = local.sql_admin_password

  identity {
    type = "SystemAssigned"
  }

  lifecycle {
    precondition {
      condition     = local.sql_admin_password != null
      error_message = "SQL administrator password could not be determined."
    }
  }

  tags = merge(local.base_tags, {
    Name = format("sql-%s-%s", local.name_prefix, var.region_key)
  })
}

resource "azurerm_mssql_database" "this" {
  count = local.sql_enabled ? 1 : 0

  name           = format("sqldb-%s-%s", local.name_prefix, var.region_key)
  server_id      = azurerm_mssql_server.this[0].id
  sku_name       = try(local.sql_cfg.sku_name, "GP_Gen5_2")
  max_size_gb    = try(local.sql_cfg.max_size_gb, 32)
  zone_redundant = false

  tags = merge(local.base_tags, {
    Name = format("sqldb-%s-%s", local.name_prefix, var.region_key)
  })
}

################################################################################
# Azure Front Door                                                             #
################################################################################

resource "azurerm_cdn_frontdoor_profile" "this" {
  count = local.front_door_enabled ? 1 : 0

  name                = lower(coalesce(try(local.front_door_cfg.profile_name, null), format("fdp-%s-%s", local.name_prefix_compact, var.region_key)))
  resource_group_name = var.resource_group_name
  sku_name            = try(local.front_door_cfg.sku_name, "Standard_AzureFrontDoor")

  tags = merge(local.base_tags, {
    Name = format("fdp-%s-%s", local.name_prefix, var.region_key)
  })
}

resource "azurerm_cdn_frontdoor_endpoint" "this" {
  count                    = local.front_door_enabled ? 1 : 0
  name                     = lower(coalesce(try(local.front_door_cfg.endpoint_name, null), format("fde-%s-%s", local.name_prefix_compact, var.region_key)))
  cdn_frontdoor_profile_id = azurerm_cdn_frontdoor_profile.this[0].id
  enabled                  = true
}

resource "azurerm_cdn_frontdoor_origin_group" "this" {
  count                    = local.front_door_enabled ? 1 : 0
  name                     = "default-origin-group"
  cdn_frontdoor_profile_id = azurerm_cdn_frontdoor_profile.this[0].id
  session_affinity_enabled = try(local.front_door_cfg.session_affinity_enabled, false)

  health_probe {
    interval_in_seconds = 60
    path                = try(local.front_door_cfg.health_probe_path, "/")
    protocol            = try(local.front_door_cfg.forwarding_protocol, "HttpsOnly") == "HttpOnly" ? "Http" : "Https"
  }

  load_balancing {
    sample_size                        = 4
    successful_samples_required        = 3
    additional_latency_in_milliseconds = 0
  }
}

resource "azurerm_cdn_frontdoor_origin" "this" {
  count                          = local.front_door_enabled ? 1 : 0
  name                           = "primary-origin"
  cdn_frontdoor_origin_group_id  = azurerm_cdn_frontdoor_origin_group.this[0].id
  host_name                      = local.front_door_cfg.origin_host
  enabled                        = true
  http_port                      = try(local.front_door_cfg.http_port, 80)
  https_port                     = try(local.front_door_cfg.https_port, 443)
  origin_host_header             = coalesce(try(local.front_door_cfg.origin_host_header, null), local.front_door_cfg.origin_host)
  priority                       = 1
  weight                         = 1000
  certificate_name_check_enabled = false
  depends_on = [
    azurerm_linux_web_app.this
  ]

  lifecycle {
    precondition {
      condition     = length(trimspace(local.front_door_cfg.origin_host)) > 0
      error_message = "front_door.origin_host must be provided."
    }
  }
}

resource "azurerm_cdn_frontdoor_route" "this" {
  count = local.front_door_enabled ? 1 : 0

  name                          = "default-route"
  cdn_frontdoor_endpoint_id     = azurerm_cdn_frontdoor_endpoint.this[0].id
  cdn_frontdoor_origin_group_id = azurerm_cdn_frontdoor_origin_group.this[0].id
  cdn_frontdoor_origin_ids      = [azurerm_cdn_frontdoor_origin.this[0].id]
  patterns_to_match             = try(local.front_door_cfg.patterns_to_match, ["/*"])
  supported_protocols           = try(local.front_door_cfg.supported_protocols, ["Http", "Https"])
  forwarding_protocol           = try(local.front_door_cfg.forwarding_protocol, "HttpsOnly")
  https_redirect_enabled        = try(local.front_door_cfg.https_redirect_enabled, true)
  link_to_default_domain        = true
}

locals {
  metadata = {
    aks = local.aks_enabled ? {
      cluster_name = azurerm_kubernetes_cluster.this[0].name
      kubelet_id   = azurerm_kubernetes_cluster.this[0].id
    } : null
    app_gateway = local.app_gw_enabled ? {
      id        = azurerm_application_gateway.this[0].id
      public_ip = azurerm_public_ip.app_gateway[0].ip_address
      subnet_id = local.app_gw_subnet_id
    } : null
    app_service = local.app_service_enabled ? {
      plan_id  = azurerm_service_plan.this[0].id
      site_id  = try(azurerm_linux_web_app.this[0].id, azurerm_windows_web_app.this[0].id)
      hostname = try(azurerm_linux_web_app.this[0].default_hostname, azurerm_windows_web_app.this[0].default_hostname)
    } : null
    load_balancer = local.load_balancer_enabled ? {
      id            = azurerm_lb.standard[0].id
      sku           = local.load_balancer_sku
      public_ip     = local.load_balancer_is_public && length(azurerm_public_ip.load_balancer) > 0 ? azurerm_public_ip.load_balancer[0].ip_address : null
      frontend_name = local.load_balancer_frontend_name
      backend_pool  = azurerm_lb_backend_address_pool.standard[0].id
    } : null
    storage = local.storage_enabled ? {
      account_name = azurerm_storage_account.this[0].name
      account_id   = azurerm_storage_account.this[0].id
    } : null
    sql = local.sql_enabled ? {
      server_id   = azurerm_mssql_server.this[0].id
      database_id = azurerm_mssql_database.this[0].id
      admin_login = azurerm_mssql_server.this[0].administrator_login
    } : null
    front_door = local.front_door_enabled ? {
      profile_id    = azurerm_cdn_frontdoor_profile.this[0].id
      endpoint_host = azurerm_cdn_frontdoor_endpoint.this[0].host_name
      origin_host   = local.front_door_cfg.origin_host
    } : null
  }
}
