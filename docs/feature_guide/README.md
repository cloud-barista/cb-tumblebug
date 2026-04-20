# CB-Tumblebug Feature Guides

This directory contains detailed documentation for specific features of CB-Tumblebug. Each guide covers the concept, architecture, and usage instructions for a key capability of the multi-cloud infrastructure management system.

## 📚 Available Guides

| Feature | Description |
|---------|-------------|
| **[Assets Backup & Restore](assets-backup-restore.md)** | Guide for backing up and restoring the PostgreSQL assets database (compute specs, images, pricing info). |
| **[Cloud-Agnostic Image](cloud-agnostic-image.md)** | Comprehensive workflow for creating CSP-agnostic custom images automatically across multiple clouds (Provision → Setup → Snapshot → Cleanup). |
| **[Credential & Connection](credential-and-connection.md)** | Guide to credentials, credential holders, and connections — multi-tenant credential isolation, connection naming, and `X-Credential-Holder` header usage. |
| **[Global DNS Management](global-dns-management.md)** | Guide for managing DNS records via AWS Route53 — Infra/Label-based IP resolution, simple and geoproximity routing, bulk operations. |
| **[High-Scale Provisioning Architecture](high-scale-provisioning-architecture.md)** | Visual analysis of the advanced architecture and optimization techniques (Rate Limiting, Parallel Processing) for massive Node provisioning. |
| **[Infra Resource Model & Lifecycle Management](infra-resource-model-and-lifecycle-management.md)** | Comprehensive guide to the Infra resource model, including the concepts of Infra, NodeGroup, and Node, as well as lifecycle control (Suspend, Resume, Reboot, Terminate, Refine) and status management. |
| **[Job Scheduler](existing-csp-res-job-scheduler.md)** | Guide for the automated task scheduler, currently supporting periodic CSP resource synchronization and registration. |
| **[Namespace & Identity](namespace-and-resource-identity.md)** | Deep dive into the logical isolation model (Namespace) and the resource identification system (User ID vs System UID vs CSP ID). |
| **[Remote Command & File Transfer](remote-command-and-file-transfer.md)** | Guide for executing remote commands and transferring files to Infra Nodes via Bastion Host with TOFU (Trust On First Use) SSH host key verification. |
| **[Resource Template Management](resource-template-management.md)** | Guide for defining, storing, and applying reusable infrastructure templates (Infra, vNet, SecurityGroup) to enable repeatable multi-cloud provisioning. |
| **[OpenStack-Type CSP Support](openstack-type-csp-support.md)** | Guide for adding OpenStack-based CSPs with 1:N Cloud Platform mapping (cloudinfo.yaml + credentials.yaml). |
| **[`make init` Workflow](make-init-workflow.md)** | Detailed walkthrough of the two-phase initialization process: OpenBao credential registration and Tumblebug asset loading, with interaction diagrams for each component. |

## 🚀 Purpose

These documents are designed to help developers and operators understand:
- **Concepts**: The "why" and "what" behind each feature.
- **Architecture**: How the feature is implemented internally (often with Mermaid diagrams).
- **Usage**: Step-by-step instructions and API references.

New feature guides will be continuously added to this directory as CB-Tumblebug evolves.
