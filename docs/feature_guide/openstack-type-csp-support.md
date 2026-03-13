# OpenStack-Type CSP Support

## Cloud Platform Concept

CB-Tumblebug uses the concept of **Cloud Platform** (also referred to as Cloud OS Type) to identify the underlying cloud technology behind each CSP (Cloud Service Provider).

For most CSPs, there is a **1:1 mapping** between the Cloud Platform and the CSP name:

| CSP Name | Cloud Platform | Mapping |
|----------|---------------|---------|
| `aws` | `aws` | 1:1 |
| `azure` | `azure` | 1:1 |
| `gcp` | `gcp` | 1:1 |

However, **OpenStack-type** CSPs have a **1:N mapping** — multiple independent CSP instances can share the same Cloud Platform:

| CSP Name | Cloud Platform | Mapping |
|----------|---------------|---------|
| `openstack` | `openstack` | 1:N |
| `openstack-new01` | `openstack` | ↑ |
| `openstack-private` | `openstack` | ↑ |

This allows organizations to manage multiple OpenStack deployments (e.g., different data centers, private clouds) as separate CSPs while sharing the same driver, icon, and platform logic.

## How It Works

When a CSP has a `cloudPlatform` field in `cloudinfo.yaml`, CB-Tumblebug resolves this value to determine the underlying platform. If not specified, the CSP name itself is used as the platform. This resolution is used for:

- **Driver selection**: Reuses the OpenStack driver (`openstack-driver-v1.0.so`)
- **Credential format**: Same credential fields as the base OpenStack
- **UI icons**: Automatically falls back to the OpenStack icon on the map
- **Spec/Image management**: Uses the same platform-level handling

## Adding a New OpenStack-Based CSP

Only **two files** need to be modified:

### 1. `assets/cloudinfo.yaml`

Add a new entry with `cloudPlatform: openstack`:

```yaml
cloud:
  # ... existing CSPs ...
  openstack-new01:
    description: My Private OpenStack Cloud
    cloudPlatform: openstack
    driver: openstack-driver-v1.0.so
    region:
      RegionOne:
        id: RegionOne
        description: My Region
        location:
          display: South Korea (Seoul)
          latitude: 37.5665
          longitude: 126.978
        zone:
        - nova
```

Key fields:
- **CSP name** (`openstack-new01`): Must start with `openstack-` for automatic platform resolution
- **`cloudPlatform: openstack`**: Explicitly declares the underlying platform
- **`driver`**: Must use `openstack-driver-v1.0.so`
- **`region`**: Define regions and zones specific to this OpenStack deployment

### 2. `init/credentials.yaml`

Add credentials with the same fields as the base `openstack` entry:

```yaml
credentialholder:
  admin:
    # ... existing CSPs ...
    openstack-new01:
      IdentityEndpoint: http://your-openstack:5000
      Username: your-user
      Password: your-password
      DomainName: default
      ProjectID: your-project-id
```

### 3. Apply Changes

```bash
make enc-cred   # Encrypt updated credentials
make init        # Reload assets and register credentials
```

The new CSP will appear in the connection list and be available for VM provisioning. The map UI will automatically display the OpenStack icon for it.

## Reference

- `cloudinfo.yaml` example: See `openstack-ex01` entry
- `template.credentials.yaml`: See the comment under the `openstack` section
