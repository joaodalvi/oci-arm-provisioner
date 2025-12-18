# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-12-17
### Added
- **Core Provisioner**: Complete Go rewrite of the original Python script.
- **Multi-Account Support**: First-class support for managing multiple OCI tenancies in parallel.
- **Dynamic AD Discovery**: Automatically finds available Availability Domains (ADs) if not specified.
- **Docker Support**: Full Dockerization with `Dockerfile`, `docker-compose.yml`, and GHCR integration.
- **Systemd Integration**: User-mode service files for identifying as a background daemon on Linux.
- **Arch Linux**: Included `PKGBUILD` for Arch User Repository (AUR) compatibility.
- **Logging**: Dual logging system (Colored Console + Structured File).
- **Graceful Shutdown**: Robust signal handling for safe termination during API calls.

### Changed
- **License**: Switched from MIT to **GPLv3**.
- **Configuration**: moved to `config.yaml` with strict validation.
- **Build System**: Introduced `Makefile` for standardized build/test/release workflows.
