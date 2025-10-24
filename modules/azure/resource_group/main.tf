locals {
  create_rg = var.create_if_missing && var.location != null && trimspace(var.location) != ""
}

data "azurerm_resource_group" "existing" {
  count = local.create_rg ? 0 : 1
  name  = var.name
}

resource "azurerm_resource_group" "this" {
  count    = local.create_rg ? 1 : 0
  name     = var.name
  location = var.location
  tags     = var.tags
}

locals {
  effective_rg = local.create_rg ? azurerm_resource_group.this[0] : data.azurerm_resource_group.existing[0]
}

output "name" {
  value = local.effective_rg.name
}

output "location" {
  value = local.effective_rg.location
}

output "id" {
  value = local.effective_rg.id
}
