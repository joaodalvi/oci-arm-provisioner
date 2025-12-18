# 📋 OCI Setup Guide

This guide explains how to get the necessary credentials from Oracle Cloud to run the **OCI ARM Provisioner**.

## 1. Create an OCI Account
Sign up for an Oracle Cloud Free Tier account at [oracle.com/cloud/free](https://www.oracle.com/cloud/free/).

## 2. Generate API Keys
You need an API Signing Key to authenticate the application.

1.  Login to the OCI Console.
2.  Go to **User Settings** (Top right profile icon -> User Settings).
3.  Click **API Keys** -> **Add API Key**.
4.  Select **Generate API Key Pair**.
5.  **Download Private Key** (`.pem` file). Save it to `~/.oci/` (e.g., `~/.oci/oci_api_key.pem`).
6.  Click **Add**.

## 3. Get Configuration Details (OCIDs)

Copy the values shown in the "Configuration File Preview" on the API Key page, or find them manually:

*   **User OCID**: Found in User Settings.
*   **Tenancy OCID**: Found in Profile -> Tenancy.
*   **Fingerprint**: Shown in the API Keys list.
*   **Region**: Your home region (e.g., `sa-saopaulo-1`, `us-ashburn-1`).

## 4. Get Instance Details

1.  Go to **Compute** -> **Instances** -> **Create Instance**.
2.  **Image**: Select "Canonical Ubuntu 22.04" (or your choice). Scroll down to "Image OCID" or verify the OS name.
3.  **Shape**: Select **Ampere (ARM)** -> `VM.Standard.A1.Flex`. Select 4 OCPUs and 24GB RAM.
4.  **Networking**: Create a **VCN** and **Subnet** if you don't have one. Copy the **Subnet OCID**.
5.  **Availability Domain**: Note the name (e.g., `QaKc:SA-SAOPAULO-1-AD-1`). You can use `"auto"` in the config to let the app find it.
6.  **SSH Key**: You must provide your **Public SSH Key** string (contents of `~/.ssh/id_rsa.pub`) to access the VM later.

## 5. Configure the Application
**Easiest Method:**
Run the interactive wizard and paste the values you gathered above:
```bash
./oci-arm-provisioner --setup
```

**Manual Method:**
Edit `config.yaml`:
```yaml
accounts:
  main:
    enabled: true
    user_ocid: "..."
    tenancy_ocid: "..."
    fingerprint: "..."
    key_file: "~/.oci/oci_api_key.pem"
    region: "sa-saopaulo-1"
    # ... other details ...
```

## 6. Run & Relax

```bash
./oci-arm-provisioner
```
The application will loop and retry until your instance is created.
