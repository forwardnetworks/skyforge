output "connection_monitors" {
  description = "IDs of network connection monitors keyed by test name."
  value = {
    for name, resource in azurerm_network_connection_monitor.this :
    name => resource.id
  }
}

