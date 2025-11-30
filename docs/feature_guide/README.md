# CB-Tumblebug Feature Guides

This directory contains detailed documentation for specific features of CB-Tumblebug. Each guide covers the concept, architecture, and usage instructions for a key capability of the multi-cloud infrastructure management system.

## 📚 Available Guides

| Feature | Description |
|---------|-------------|
| **[Assets Backup & Restore](assets-backup-restore.md)** | Guide for backing up and restoring the PostgreSQL assets database (VM specs, images, pricing info). |
| **[Cloud-Agnostic Image](cloud-agnostic-image.md)** | Comprehensive workflow for creating CSP-agnostic custom images automatically across multiple clouds (Provision → Setup → Snapshot → Cleanup). |
| **[High-Scale Provisioning Architecture](high-scale-provisioning-architecture.md)** | Visual analysis of the advanced architecture and optimization techniques (Rate Limiting, Parallel Processing) for massive VM provisioning. |
| **[MCI/VM Lifecycle Management](mci-vm-lifecycle-management.md)** | Comprehensive guide to the lifecycle control (Suspend, Resume, Reboot, Terminate, Refine) and status management of Multi-Cloud Infrastructures and VMs. |
| **[Job Scheduler](existing-csp-res-job-scheduler.md)** | Guide for the automated task scheduler, currently supporting periodic CSP resource synchronization and registration. |
| **[Namespace & Identity](namespace-and-resource-identity.md)** | Deep dive into the logical isolation model (Namespace) and the resource identification system (User ID vs System UID vs CSP ID). |

## 🚀 Purpose

These documents are designed to help developers and operators understand:
- **Concepts**: The "why" and "what" behind each feature.
- **Architecture**: How the feature is implemented internally (often with Mermaid diagrams).
- **Usage**: Step-by-step instructions and API references.

New feature guides will be continuously added to this directory as CB-Tumblebug evolves.
