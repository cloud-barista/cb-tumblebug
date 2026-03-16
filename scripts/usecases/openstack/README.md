# OpenStack (DevStack) on AWS for CB-Tumblebug

Deploy a single-node OpenStack environment on an AWS bare-metal instance and register it as a new CSP in CB-Tumblebug.

## Prerequisites

- AWS VM with **bare-metal** instance type (e.g., `m5.metal`) for KVM support
- Ubuntu 22.04 or 24.04
- At least 50 GiB disk space
- SSH access to the VM

## Quick Start

### 1. Install DevStack

SSH into the AWS VM and run:

```bash
./1.installDevStack.sh
```

Options:
- `--password PASSWORD` — OpenStack admin password (default: `cbtumblebug`)
- `--branch BRANCH` — OpenStack release branch (default: `stable/2024.2`)

This takes **15–30 minutes**. After completion, you'll have a working OpenStack with:
- Keystone (identity)
- Nova (compute) with KVM
- Glance (images) with Ubuntu 22.04 cloud image
- Neutron (networking)
- Cinder (block storage)
- Horizon dashboard

### 2. Get Registration Info

```bash
./2.getRegistrationInfo.sh
```

Options:
- `--csp-name NAME` — CSP instance name for CB-Tumblebug (default: `openstack-devstack`)

This outputs:
- **cloudinfo.yaml snippet** — copy into `cb-tumblebug/assets/cloudinfo.yaml`
- **credentials.yaml snippet** — copy into `cb-tumblebug/init/credentials.yaml`
- API connectivity test results

### 3. Register in CB-Tumblebug

1. Add the generated snippets to the respective files
2. Open AWS Security Group ports: **80, 5000, 8774, 9292, 9696**
3. Run:
   ```bash
   make enc-cred && make init
   ```

The new CSP will appear in the connection list and can be used for VM provisioning.

## Architecture

```
CB-Tumblebug (:1323)
    ↓ REST API
CB-Spider (:1024) → openstack-driver-v1.0.so
    ↓ OpenStack API
AWS m5.metal VM
    ├─ Keystone (:80/identity)
    ├─ Nova (:8774)
    ├─ Glance (:9292)
    ├─ Neutron (:9696)
    └─ KVM → nested VMs
```

## Notes

- **Cost**: `m5.metal` is ~$4.6/hour. Terminate when not in use.
- **Performance**: KVM runs natively on bare-metal. If using non-metal instances, change `LIBVIRT_TYPE=qemu` in `local.conf` (very slow).
- **Persistence**: DevStack is development-only. Data does not persist across reboots. Run `./opt/stack/devstack/stack.sh` to restart after reboot.
- **Security**: The default `cbtumblebug` password is for testing only. Change it for any non-local deployment.
