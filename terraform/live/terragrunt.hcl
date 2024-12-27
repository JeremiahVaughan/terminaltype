locals {
  terraform_state_bucket_region = get_env("TF_VAR_terraform_state_bucket_region")
  region     = get_env("TF_VAR_infra_aws_region")
  access_key = get_env("TF_VAR_infra_aws_key_id")
  secret_key = get_env("TF_VAR_infra_aws_secret")
  api_token = get_env("TF_VAR_infra_cloudflare_api_token")
  statuscake_key = get_env("TF_VAR_statuscake_key")
}

generate "providers" {
  path      = "generated_providers.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region     = "${local.region}"
  access_key = "${local.access_key}"
  secret_key = "${local.secret_key}"
}

provider "statuscake" {
  api_token = "${local.statuscake_key}"
}

provider "cloudflare" {
  api_token = "${local.api_token}"
}
EOF
}


generate "versions" {
  path      = "generated_versions.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    cloudflare = {
      source = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
    statuscake = {
      source = "StatusCakeDev/statuscake"
      version = "~> 2.2.2"  # This version must stay in sync with the sub module called endpoint in the statuscake parent module.
    }
  }
}
EOF
}

generate "backend" {
  path      = "generated_backend.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
terraform {
  backend "s3" {
    bucket = "strength-gadget-terraform-state"
    key            = "${path_relative_to_include()}/terraform.tfstate"
    region         = "${local.terraform_state_bucket_region}"
    encrypt        = true
    dynamodb_table = "strength-gadget-terraform-state"
  }
}
EOF
}
