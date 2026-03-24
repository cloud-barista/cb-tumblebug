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
- `--branch BRANCH` — OpenStack release branch (default: `stable/2025.2`)

This takes **15–30 minutes**. After completion, you'll have a working OpenStack with:
- Keystone (identity)
- Nova (compute) with KVM
- Glance (images) with Ubuntu 22.04 cloud image
- Neutron (networking)
- Cinder (block storage)
- Horizon dashboard
- Placeholder/alias entries for CB-Spider compatibility (load-balancer, shared-file-system)

### 2. Get Registration Info

```bash
./2.getRegistrationInfo.sh
```

Options:
- `--csp-name NAME` — CSP instance name for CB-Tumblebug (default: `openstack-devstack`)

This outputs:
- **cloudinfo.yaml snippet** — copy into `cb-tumblebug/assets/cloudinfo.yaml`
- **credentials.yaml snippet** — copy into `~/.cloud-barista/credentials.yaml`
- API connectivity test results

### 3. Register in CB-Tumblebug

1. Add the generated snippets to the respective files
2. Open AWS Security Group ports: **80, 8774, 9292, 9696**
3. Run:
   ```bash
   make enc-cred && make init
   ```

The new CSP will appear in the connection list and can be used for VM provisioning.

### 4. Update Endpoints (after IP change)

If the VM's public IP changes (e.g., after suspend/resume or stop/start):

```bash
./3.updateEndpoints.sh --csp-name openstack-devstack
```

This updates all OpenStack service catalog endpoints to the new public IP and outputs an updated `credentials.yaml` snippet. After running, update `~/.cloud-barista/credentials.yaml` and re-run `make enc-cred && make init`.

> **Tip**: Use AWS Elastic IP to avoid IP changes altogether.

### 5. Clean / Rollback (after failed install)

If the installation failed or you want to start fresh:

```bash
./4.cleanDevStack.sh         # Keep source for faster re-install
./4.cleanDevStack.sh --full  # Remove everything including source
```

After cleanup, run `./1.installDevStack.sh` again to re-install.

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

## Troubleshooting

### "No suitable endpoint could be found in the service catalog"

CB-Spider's OpenStack driver (gophercloud v2) requires ALL service clients during connection initialization. It expects these service types in the Keystone service catalog:

| Service | gophercloud v2 Type | Aliases (also matched) | DevStack 2025.2 | Fix |
|---------|---------------------|----------------------|-----------------|-----|
| Cinder  | `block-storage`     | `volumev3`, `volumev2`, `volume`, `block-store` | `block-storage` | None needed (direct match) |
| Octavia | `load-balancer`     | (none)               | Not installed   | Placeholder |
| Manila  | `shared-file-system`| `sharev2`, `share`   | Not installed   | Placeholder |

The install script automatically creates placeholder entries for Octavia and Manila. If you see this error:

```bash
# SSH into the DevStack VM and run:
./2.getRegistrationInfo.sh    # Ensures placeholders/aliases exist
# or
./3.updateEndpoints.sh        # Also ensures placeholders/aliases exist
```

### "Authentication failed"

Verify the credentials in `~/.cloud-barista/credentials.yaml` match the DevStack admin password. The `ProjectID` must match the actual admin project ID (run `openstack project show admin -f value -c id` on the DevStack VM).
