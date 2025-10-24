output "path_ids" {
  description = "Network Insights Path IDs created for the region."
  value = {
    for name, resource in aws_ec2_network_insights_path.this :
    name => resource.id
  }
}

output "analysis_ids" {
  description = "Network Insights Analysis IDs keyed by path name."
  value = {
    for name, resource in aws_ec2_network_insights_analysis.this :
    name => resource.id
  }
}
