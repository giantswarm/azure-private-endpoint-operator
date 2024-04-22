# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2024-04-22

### Added

- Add toleration for `node.cluster.x-k8s.io/uninitialized` taint.
- Add node affinity to prefer schedule to `control-plane` nodes.

## [0.2.0] - 2024-03-21

### Added

- Add a new feature that injects private endpoint to workload clusters for WC-to-MC ingress communication for private MCs. 

## [0.1.1] - 2024-01-22

### Changed

- Configure `gsoci.azurecr.io` as the default container image registry.
- Add toggle for PSPs.

## [0.1.0] - 2023-07-21

### Fixed

- Add required values for pss policies.

### Added

- Add `privatelinks` package with `Scope` object that is providing functionality to access and update private links info in AzureCluster CR.
- Add custom Makefile
- Add CircleCI config
- Add this changelog
- Add Helm chart
- Add `privateendpoints` package with `scope` object that is providing functionality to access and update private endpoints in AzureCluster CR.
- Add private endpoints reconciler Service
- Add AzureCluster controller

### Changed

- Updated Dockerfile

[Unreleased]: https://github.com/giantswarm/azure-private-endpoint-operator/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/giantswarm/azure-private-endpoint-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/azure-private-endpoint-operator/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/giantswarm/azure-private-endpoint-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/azure-private-endpoint-operator/releases/tag/v0.1.0
