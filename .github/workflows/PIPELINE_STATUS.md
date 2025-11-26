# CI/CD Pipeline Status

## Last Update: 2025-11-26

### Recent Fixes Applied

✅ **Cloud-init POSIX Compatibility Fixed**
- Fixed bashism (here-string `<<<`) in cloud-init.yaml
- Cloud-init runs with `/bin/sh` (dash on Ubuntu), not bash
- Changed to POSIX-compatible `echo | pipe` syntax
- deployment-workflows@fix/ssh-connectivity-debugging updated: commit bccebbb

✅ **Cloud-init User Timing Fixed**
- Fixed "Unknown user: ubuntu" error in write_files
- Added `defer: true` to defer file writes until ubuntu user exists
- write_files runs before user creation without defer flag
- deployment-workflows@fix/ssh-connectivity-debugging updated: commit 05081a0

✅ **Added unzip package**
- Oracle Wallet extraction requires unzip
- Added to cloud-init packages list
- deployment-workflows@fix/ssh-connectivity-debugging updated: commit 1f0bc93

✅ **Terraform Backend Issue Fixed**
- Removed HTTP backend configuration
- Using local backend with GitHub Actions cache
- deployment-workflows@main updated: commit 0b8a14c

✅ **SSH Public Key Heredoc Syntax Fixed**
- Changed from double quotes to heredoc (<<-EOT)
- Fixed multi-line string validation error

✅ **Database Storage Configuration Fixed**
- Removed data_storage_size_in_tbs attribute
- Always Free tier has fixed 20GB storage
- deployment-workflows@main updated: commit 7470db7

✅ **OCIR Variables Added**
- Added ocir_region and ocir_namespace variables
- Pass OCIR secrets to cloud-init template
- Added missing ORACLE_DB_ADMIN_PASSWORD secret
- deployment-workflows@main updated: commit a642be5

✅ **VNIC Query Fixed**
- Fixed "Unsupported attribute vnic_id" error
- Added oci_core_vnic_attachments data source
- Properly query VNIC to get public IP
- deployment-workflows@main updated: commit b756d34

✅ **Centralized Terraform Infrastructure**
- Terraform code in deployment-workflows repo
- Workflow checks out deployment-workflows for infrastructure

✅ **Full CD Pipeline Enabled**
- Staging deployment active on develop branch
- Integration tests enabled
- Production deployment configured for main branch

✅ **Oracle Instant Client Added to Cloud-init**
- Pipeline failed at "Validate Oracle Wallet Connectivity" - sqlplus not installed
- Added Oracle Instant Client 21.1 (Basic Lite + SQL*Plus) to cloud-init
- Also added libaio1 package (Oracle Instant Client dependency)
- deployment-workflows@fix/ssh-connectivity-debugging updated: commit 70badc9

### Next Test
Testing Oracle Instant Client installation for sqlplus migrations - commit 70badc9
