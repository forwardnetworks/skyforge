# Sample overrides for deploying only the us-east-1 AWS region during initial testing.
aws_regions = {
  "us-east-1" = var.aws_regions["us-east-1"]
}
