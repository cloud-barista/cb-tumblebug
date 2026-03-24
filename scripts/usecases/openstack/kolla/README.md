# OpenStack (Kolla-Ansible) on AWS for CB-Tumblebug

Deploy a **production-grade** single-node OpenStack environment on an AWS bare-metal instance using Kolla-Ansible, and register it as a new CSP in CB-Tumblebug.

## DevStack vs Kolla-Ansible

| | **DevStack** (`../`) | **Kolla-Ansible** (this) |
|---|---|---|
| **Purpose** | Development/testing | Production-grade |
| **Services** | OS processes (screen) | Docker containers |
| **Reboot** | ❌ Manual restart needed | ✅ Auto-restart |
| **Stability** | Fragile | Stable |
| **Install Time** | 15–30 min | 20–40 min |
| **CB-Spider** | Needs placeholder services | Placeholder services (same approach, auto-created) |
| **Upgrade** | Not supported | Rolling upgrade |

**Recommendation:** Use Kolla-Ansible for persistent test environments. Use DevStack only for quick one-off testing.

## Prerequisites

- AWS VM with **bare-metal** instance type (e.g., `m5.metal`) for KVM support
- Ubuntu 22.04
- At least 50 GiB disk space
- SSH access to the VM
- AWS Security Group: open ports **5000, 8774, 9292, 9696, 8776, 80**

## Quick Start

### 1. Install Kolla-Ansible

SSH into the AWS VM and run:

```bash
./1.installKolla.sh
```

Options:
- `--password PASSWORD` — OpenStack admin password (default: `cbtumblebug`)
- `--release RELEASE` — OpenStack release (default: `2025.2`)
- `--csp-name NAME` — CSP name for CB-Tumblebug (default: `openstack-kolla`)
- `--latitude LAT` — Latitude for location info
- `--longitude LON` — Longitude for location info
- `--location DISPLAY` — Display name for location

This takes **20–40 minutes** (Docker image pulls + Ansible deployment). After completion:
- All OpenStack services run as Docker containers with auto-restart
- Keystone (identity) on port **5000**
- Nova (compute) on port **8774**
- Glance (images) on port **9292**
- Neutron (networking) on port **9696**
- Cinder (block storage) on port **8776**
- Horizon (dashboard) on port **80**

### 2. Get Registration Info

```bash
./2.getRegistrationInfo.sh
```

Options:
- `--csp-name NAME` — CSP instance name (default: `openstack-kolla`)
- `--latitude LAT` / `--longitude LON` / `--location DISPLAY` — Location info

### 3. Register in CB-Tumblebug

1. Add the generated snippets to:
   - `~/.cloud-barista/credentials.yaml`
   - `cb-tumblebug/assets/cloudinfo.yaml`
2. Open AWS Security Group ports: **5000, 8774, 9292, 9696, 8776**
3. Run:
   ```bash
   make enc-cred && make init
   ```

### 4. Update Endpoints (after IP change)

If the VM's public IP changes:

```bash
./3.updateEndpoints.sh --csp-name openstack-kolla
```

> **Tip**: Use AWS Elastic IP to avoid IP changes.

### 5. Clean / Rollback

```bash
./4.cleanKolla.sh         # Keep venv for faster re-deploy
./4.cleanKolla.sh --full  # Remove everything including Docker images
```

## Architecture

```
CB-Tumblebug (:1323)
    ↓ REST API
CB-Spider (:1024) → openstack-driver-v1.0.so
    ↓ OpenStack API (standard ports)
AWS m5.metal VM (Docker containers)
    ├─ kolla_keystone    (:5000)
    ├─ kolla_nova        (:8774)
    ├─ kolla_glance      (:9292)
    ├─ kolla_neutron     (:9696)
    ├─ kolla_cinder      (:8776)
    ├─ kolla_horizon     (:80)
    └─ KVM → nested VMs
```

## Key Differences from DevStack

### Endpoint Format
- **DevStack 2025.2**: Apache reverse proxy, path-based (`http://IP/compute/v2.1`)
- **Kolla-Ansible**: Standard ports (`http://IP:8774/v2.1`)

### CB-Spider Compatibility
- **DevStack**: Requires placeholder entries for Octavia, Manila (auto-created by scripts). Cinder `block-storage` is matched directly by gophercloud v2 ServiceTypeAliases.
- **Kolla-Ansible**: Same placeholder approach — Octavia/Manila are not deployed. gophercloud v2 matches `block-storage` directly (no `volumev3` alias needed) and `shared-file-system` or `sharev2` via aliases.

### Persistence
- **DevStack**: Services die on reboot, must re-run `stack.sh`
- **Kolla-Ansible**: Docker containers with restart policy, auto-recover on reboot

## Troubleshooting

### Services not starting after reboot
```bash
# Check container status
docker ps | grep kolla

# Restart all Kolla containers
docker restart $(docker ps -a --filter "name=kolla" -q)
```

### "Authentication failed"
Verify the password in `~/.cloud-barista/credentials.yaml` matches:
```bash
grep "keystone_admin_password" /etc/kolla/passwords.yml
```

### Re-deploy without full reinstall
```bash
source /opt/kolla-venv/bin/activate
kolla-ansible -i /opt/kolla-config/all-in-one deploy
```

## Notes

- **Cost**: `m5.metal` is ~$4.6/hour. Use Elastic IP + stop/start to save costs.
- **Performance**: KVM runs natively on bare-metal.
- **Storage**: Cinder uses a 50 GiB loopback LVM volume by default.
- **CB-Spider**: Placeholder service catalog entries are auto-created for Octavia (`load-balancer`) and Manila (`shared-file-system`). gophercloud v2 uses ServiceTypeAliases, so Cinder `block-storage` is matched directly (no `volumev3` alias needed). Deploying Octavia/Manila on AWS bare-metal is impractical (Octavia needs amphora images + management networks; Manila needs service VMs).
- **Security**: The default `cbtumblebug` password is for testing only.
