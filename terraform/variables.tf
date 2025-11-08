# ===================================
# ORACLE CLOUD AUTHENTICATION
# ===================================
# These will be provided via GitHub Secrets

variable "tenancy_ocid" {
  description = "Oracle Cloud Tenancy OCID"
  type        = string
}

variable "user_ocid" {
  description = "Oracle Cloud User OCID"
  type        = string
}

variable "fingerprint" {
  description = "API Key Fingerprint"
  type        = string
}

variable "private_key" {
  description = "API Private Key (PEM format)"
  type        = string
  sensitive   = true
}

variable "region" {
  description = "Oracle Cloud Region (e.g., us-ashburn-1)"
  type        = string
  default     = "us-ashburn-1"
}

variable "compartment_ocid" {
  description = "Compartment OCID (usually same as tenancy for root compartment)"
  type        = string
}

# ===================================
# COMPUTE CONFIGURATION
# ===================================

variable "instance_shape" {
  description = "Compute instance shape (Always Free: VM.Standard.A1.Flex)"
  type        = string
  default     = "VM.Standard.A1.Flex"
}

variable "instance_ocpus" {
  description = "Number of OCPUs (Always Free: up to 4)"
  type        = number
  default     = 4
}

variable "instance_memory_gb" {
  description = "Memory in GB (Always Free: up to 24GB)"
  type        = number
  default     = 24
}

variable "instance_boot_volume_size_gb" {
  description = "Boot volume size in GB"
  type        = number
  default     = 50
}

# ===================================
# DATABASE CONFIGURATION
# ===================================

variable "db_admin_password" {
  description = "Autonomous Database admin password"
  type        = string
  sensitive   = true
}

variable "db_app_user" {
  description = "Application database user"
  type        = string
  default     = "payment_service"
}

variable "db_app_password" {
  description = "Application database password"
  type        = string
  sensitive   = true
}

# ===================================
# APPLICATION CONFIGURATION
# ===================================

variable "environment" {
  description = "Environment name (staging, production)"
  type        = string
  default     = "staging"
}

variable "epx_mac" {
  description = "EPX Browser Post MAC key"
  type        = string
  sensitive   = true
}

variable "cron_secret" {
  description = "Secret for cron endpoint authentication"
  type        = string
  sensitive   = true
}

# ===================================
# NETWORKING
# ===================================

variable "vcn_cidr" {
  description = "VCN CIDR block"
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidr" {
  description = "Subnet CIDR block"
  type        = string
  default     = "10.0.1.0/24"
}

# ===================================
# SSH ACCESS
# ===================================

variable "ssh_public_key" {
  description = "SSH public key for instance access"
  type        = string
}
