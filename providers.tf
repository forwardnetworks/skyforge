provider "aws" {
  alias      = "us-east-1"
  region     = "us-east-1"
  sts_region = "us-east-1"
  default_tags {
    tags = var.default_tags
  }
}

provider "aws" {
  alias      = "global"
  region     = "us-west-2"
  sts_region = "us-east-1"
  default_tags {
    tags = var.default_tags
  }
}

provider "aws" {
  alias      = "eu-central-1"
  region     = "eu-central-1"
  sts_region = "us-east-1"
  default_tags {
    tags = var.default_tags
  }
}

provider "aws" {
  alias      = "ap-northeast-1"
  region     = "ap-northeast-1"
  sts_region = "us-east-1"
  default_tags {
    tags = var.default_tags
  }
}

provider "aws" {
  alias      = "me-south-1"
  region     = "me-south-1"
  sts_region = "us-east-1"
  default_tags {
    tags = var.default_tags
  }
}

provider "azurerm" {
  features {}
  use_cli = true
}

provider "google" {
  alias  = "us_central1"
  region = "us-central1"
}

provider "google" {
  alias  = "europe_west1"
  region = "europe-west1"
}

provider "google" {
  alias  = "asia_southeast1"
  region = "asia-southeast1"
}

provider "google" {
  alias  = "me_west1"
  region = "me-west1"
}

provider "google" {
  region = "us-central1"
}

provider "google-beta" {
  alias  = "us_central1"
  region = "us-central1"
}

provider "google-beta" {
  alias  = "europe_west1"
  region = "europe-west1"
}

provider "google-beta" {
  alias  = "asia_southeast1"
  region = "asia-southeast1"
}

provider "google-beta" {
  alias  = "me_west1"
  region = "me-west1"
}

provider "google-beta" {
  region = "us-central1"
}

provider "random" {}
provider "tls" {}
provider "local" {}
