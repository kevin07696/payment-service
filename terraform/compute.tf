# ===================================
# COMPUTE INSTANCE (Always Free Ampere A1)
# ===================================

# Get the latest Ubuntu ARM image
data "oci_core_images" "ubuntu_arm_images" {
  compartment_id           = var.compartment_ocid
  operating_system         = "Canonical Ubuntu"
  operating_system_version = "22.04"
  shape                    = var.instance_shape
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"
}

# Generate SSH key pair if not provided
resource "tls_private_key" "ssh_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Compute Instance
resource "oci_core_instance" "payment_instance" {
  compartment_id      = var.compartment_ocid
  availability_domain = data.oci_identity_availability_domains.ads.availability_domains[0].name
  display_name        = "payment-${var.environment}-instance"
  shape               = var.instance_shape

  shape_config {
    ocpus         = var.instance_ocpus
    memory_in_gbs = var.instance_memory_gb
  }

  source_details {
    source_type             = "image"
    source_id               = data.oci_core_images.ubuntu_arm_images.images[0].id
    boot_volume_size_in_gbs = var.instance_boot_volume_size_gb
  }

  create_vnic_details {
    subnet_id        = oci_core_subnet.payment_subnet.id
    assign_public_ip = true
    display_name     = "payment-${var.environment}-vnic"
  }

  metadata = {
    ssh_authorized_keys = coalesce(var.ssh_public_key, tls_private_key.ssh_key.public_key_openssh)
    user_data = base64encode(templatefile("${path.module}/cloud-init.yaml", {
      db_connection_string = oci_database_autonomous_database.payment_db.connection_strings[0].profiles[0].value
      db_user              = var.db_app_user
      db_password          = var.db_app_password
      epx_mac              = var.epx_mac
      cron_secret          = var.cron_secret
      environment          = var.environment
    }))
  }

  lifecycle {
    ignore_changes = [
      metadata, # Prevent recreation on metadata changes
    ]
  }
}

# Get public IP
data "oci_core_vnic" "payment_instance_vnic" {
  vnic_id = oci_core_instance.payment_instance.create_vnic_details[0].vnic_id
}

# Save SSH private key to file
resource "local_sensitive_file" "ssh_private_key" {
  content  = coalesce(var.ssh_public_key != "" ? "" : tls_private_key.ssh_key.private_key_openssh, "")
  filename = "${path.module}/oracle-staging-key"
  file_permission = "0600"
}
