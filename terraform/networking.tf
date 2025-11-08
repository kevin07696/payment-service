# ===================================
# NETWORKING RESOURCES
# ===================================

# Get availability domains
data "oci_identity_availability_domains" "ads" {
  compartment_id = var.tenancy_ocid
}

# Virtual Cloud Network (VCN)
resource "oci_core_vcn" "payment_vcn" {
  compartment_id = var.compartment_ocid
  cidr_block     = var.vcn_cidr
  display_name   = "payment-${var.environment}-vcn"
  dns_label      = "payment${var.environment}"
}

# Internet Gateway
resource "oci_core_internet_gateway" "payment_igw" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.payment_vcn.id
  display_name   = "payment-${var.environment}-igw"
  enabled        = true
}

# Route Table
resource "oci_core_route_table" "payment_rt" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.payment_vcn.id
  display_name   = "payment-${var.environment}-rt"

  route_rules {
    network_entity_id = oci_core_internet_gateway.payment_igw.id
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
  }
}

# Security List
resource "oci_core_security_list" "payment_seclist" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.payment_vcn.id
  display_name   = "payment-${var.environment}-seclist"

  # Allow all outbound
  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  # Allow SSH (22)
  ingress_security_rules {
    protocol = "6" # TCP
    source   = "0.0.0.0/0"
    tcp_options {
      min = 22
      max = 22
    }
  }

  # Allow gRPC (8080)
  ingress_security_rules {
    protocol = "6" # TCP
    source   = "0.0.0.0/0"
    tcp_options {
      min = 8080
      max = 8080
    }
  }

  # Allow HTTP Cron (8081)
  ingress_security_rules {
    protocol = "6" # TCP
    source   = "0.0.0.0/0"
    tcp_options {
      min = 8081
      max = 8081
    }
  }

  # Allow ICMP (ping)
  ingress_security_rules {
    protocol = "1" # ICMP
    source   = "0.0.0.0/0"
  }
}

# Subnet
resource "oci_core_subnet" "payment_subnet" {
  compartment_id      = var.compartment_ocid
  vcn_id              = oci_core_vcn.payment_vcn.id
  cidr_block          = var.subnet_cidr
  display_name        = "payment-${var.environment}-subnet"
  dns_label           = "payment${var.environment}"
  route_table_id      = oci_core_route_table.payment_rt.id
  security_list_ids   = [oci_core_security_list.payment_seclist.id]
  prohibit_public_ip_on_vnic = false
}
