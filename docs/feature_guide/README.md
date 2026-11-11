# CB-Tumblebug Feature Guides

> 🌐 **Language**: **English** | [한국어](ko/README.md)

This directory contains detailed documentation for specific features of CB-Tumblebug. Each guide covers the concept, architecture, and usage instructions for a key capability of the multi-cloud infrastructure management system.

## 📚 Available Guides

| Feature | Description |
|---------|-------------|
| **[Assets Backup & Restore](assets-backup-restore.md)** | Guide for backing up and restoring the PostgreSQL assets database (compute specs, images, pricing info). |
| **[Cloud-Agnostic Image](cloud-agnostic-image.md)** | Comprehensive workflow for creating CSP-agnostic custom images automatically across multiple clouds (Provision → Setup → Snapshot → Cleanup). |
| **[Credential & Connection](credential-and-connection.md)** | Guide to credentials, credential holders, and connections: multi-tenant credential isolation, connection naming, and `X-Credential-Holder` header usage. |
| **[Global DNS Management](global-dns-management.md)** | Guide for managing DNS records via AWS Route53: Infra/Label-based IP resolution, simple and geoproximity routing, bulk operations. |
| **[High-Scale Provisioning Architecture](high-scale-provisioning-architecture.md)** | Problem-driven architecture guide covering the 4 parts and 10 core bottlenecks (Challenges 1-10) solved to provision and manage 10,000+ VMs across 10 CSPs and 146 regions. |
| **[Infra Provisioning](infra-provisioning.md)** | End-to-end guide to creating infrastructure: request structure, partial-failure policy, auto-created shared resources, labels, post-deployment commands (phases, targeting, async streaming), scaling, verification, and cleanup. |
| **[Infra Resource Model & Lifecycle Management](infra-resource-model-and-lifecycle-management.md)** | Comprehensive guide to the Infra resource model, including the concepts of Infra, NodeGroup, and Node, as well as lifecycle control (Suspend, Resume, Reboot, Terminate, Refine) and status management. |
| **[Job Scheduler](existing-csp-res-job-scheduler.md)** | Guide for the automated task scheduler, currently supporting periodic CSP resource synchronization and registration. |
| **[`make init` Workflow](make-init-workflow.md)** | Detailed walkthrough of the two-phase initialization process: OpenBao credential registration and Tumblebug asset loading, with interaction diagrams for each component. |
| **[Namespace & Identity](namespace-and-resource-identity.md)** | Deep dive into the logical isolation model (Namespace) and the resource identification system (User ID vs System UID vs CSP ID). |
| **[Network Status Management](network-status-management.md)** | Guide for multi-cloud network resource status modeling (Kubernetes-style Conditions) and lifecycle verification for VNet, Subnet, and VPN. |
| **[Notifier (ntfy)](notifier-ntfy.md)** | Guide to the ntfy-based notification channel: provisioning a managed server (self-signed or Route53 + Let's Encrypt), registering an existing one, write/read tokens, subscriber accounts for people, publishing and receiving messages. |
| **[Init Script (first-boot provisioning)](init-script.md)** | Guide to `initScript`: CB-Tumblebug-rendered cloud-init user data with the SSH account, reusable script templates (shared in `system`), ordered steps and result reporting through the Notifier. |
| **[OpenStack-Type CSP Support](openstack-type-csp-support.md)** | Guide for adding OpenStack-based CSPs with 1:N Cloud Platform mapping (cloudinfo.yaml + credentials.yaml). |
| **[RDBMS Management](rdbms-management.md)** | Guide for provisioning, configuring, and managing multi-cloud managed relational database (RDBMS) instances and Bastion access. |
| **[Remote Command & File Transfer](remote-command-and-file-transfer.md)** | Guide for executing remote commands and transferring files to Infra Nodes via Bastion Host with TOFU (Trust On First Use) SSH host key verification. |
| **[Resource Reconciliation](resource-reconciliation.md)** | Guide for declarative resource reconciliation, diagnostic state machine, self-healing, and orphaned metadata pruning (`PUT .../reconcile`, `POST .../prune`). |
| **[Resource Template Management](resource-template-management.md)** | Guide for defining, storing, and applying reusable infrastructure templates (Infra, vNet, SecurityGroup) to enable repeatable multi-cloud provisioning. |
| **[Shared Resource Management](shared-resource-management.md)** | Guide to shared resources (VNet, SecurityGroup, SSHKey): lifecycle, release, and recovering from DependencyViolation caused by orphaned CSP resources. |
| **[Spec & Image Recommendation](spec-image-recommendation.md)** | Guide for recommending compute specs, searching images, reviewing spec-image pairs, and finding alternative configurations across CSPs (`recommendSpec`, `searchImage`, `recommendAlternativeNodeConfig`). |
| **[Storage Status Management](storage-status-management.md)** | Guide for DataDisk and Object Storage status management, attachment/detachment, and lifecycle verification. |

## 🚀 Purpose

These documents are designed to help developers and operators understand:
- **Concepts**: The "why" and "what" behind each feature.
- **Architecture**: How the feature is implemented internally (often with Mermaid diagrams).
- **Usage**: Step-by-step instructions and API references.

New feature guides will be continuously added to this directory as CB-Tumblebug evolves.
