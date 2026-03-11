# CB-Spider Cloud Driver Libs

This directory contains configuration files originally from
[cb-spider/cloud-driver-libs](https://github.com/cloud-barista/cb-spider/tree/master/cloud-driver-libs),
managed locally within CB-Tumblebug.

## Why not in the container image?

Starting from CB-Spider 0.12.8, the Docker image no longer includes `cloud-driver-libs/`
(static mode optimization). However, these files are still required at runtime for:

- **VM provisioning** — cloud-init scripts (`.cloud-init-*`) are injected during VM creation
- **Root disk configuration** — `cloudos_meta.yaml` provides CSP-specific disk type/size metadata
- **CSP registry** — `cloudos.yaml` lists supported cloud providers
- **Region metadata** — `region/` contains per-CSP region information

Since CB-Tumblebug administrators may need to customize some of these files
(e.g., cloud-init scripts for specific deployment environments or CSP-specific settings),
they are maintained here rather than embedded in the container image.

This directory is mounted into the CB-Spider container via `docker-compose.yaml`:

```yaml
volumes:
  - ./assets/spider/:/root/go/src/github.com/cloud-barista/cb-spider/cloud-driver-libs/
```

## Updating

When upgrading the CB-Spider version in `docker-compose.yaml`, update this directory
by copying from the corresponding version of cb-spider:

```bash
# Example: sync with cb-spider source
cp -r <cb-spider-source>/cloud-driver-libs/* assets/spider/
```

## Directory Structure

| Path | Description |
|------|-------------|
| `cloudos_meta.yaml` | CSP metadata (credentials, disk types, region defaults) |
| `cloudos.yaml` | List of supported cloud providers |
| `.cloud-init-*/` | Per-CSP cloud-init scripts for VM initialization |
| `region/` | Per-CSP region display names (not used by CB-TB; included for CB-Spider AdminWeb) |
