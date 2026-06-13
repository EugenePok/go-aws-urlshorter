variable "project" {
  description = "Name prefix applied to resources and tag"
  type        = string
  default     = "url-shortener"
}

variable "region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Number of Availability Zones to spread across (>= 2 for HA)"
  type        = number
  default     = 2
  validation {
    condition     = var.az_count >= 2
    error_message = "az_count must be at least 2 for high availability"
  }
}

variable "single_nat_gateway" {
  description = <<-EOT
    If true, one shared NAT gateway serves all private subnet
    cheaper but a single AZ failure cuts outbound internet for every private subnet
    If false, one NAT gateway per AZ (HA, but each NAT costs ~$32/month + data)
EOT
  type        = bool
  default     = true
}

variable "app_port" {
  description = "Port the API container listens on"
  type        = number
  default     = 8080
}
