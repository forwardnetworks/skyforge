output "connectivity_tests" {
  description = "Connectivity test resource IDs keyed by test name."
  value = {
    for name, resource in google_network_management_connectivity_test.this :
    name => resource.id
  }
}

