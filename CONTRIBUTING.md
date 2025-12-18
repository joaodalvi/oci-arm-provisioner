# Contributing to OCI ARM Provisioner

First off, thanks for taking the time to contribute! 🎉

The following is a set of guidelines for contributing to this project. These are mostly guidelines, not rules. Use your best judgment, and feel free to propose changes to this document in a pull request.

## Code of Conduct

This project is open source. Please be respectful and professional in all interactions.

## How Can I Contribute?

### Reporting Bugs

This section guides you through submitting a bug report.
*   **Use the issue search** to see if the problem has already been reported.
*   **Check if the issue is reproducible** with the latest version.
*   **Fill out the bug report form** (if available) or describe the bug in detail, including:
    *   OS / Arch (e.g. Ubuntu 22.04 / AMD64)
    *   Config (sans secrets)
    *   Logs

### Suggesting Enhancements

Enhancement suggestions are welcome!
*   **Explain why** this enhancement would be useful.
*   **Provide examples** of how it would work.

### Pull Requests

1.  **Fork the repo** and create your branch from `main`.
2.  **Test your code**: Run `make test` to ensure it passes.
3.  **Lint your code**: Use `go vet` or `golangci-lint`.
4.  **Document**: Add comments and update `README.md` if needed.
5.  **Sign your work**: Ensure you have configured your git email.

## Development Setup

1.  **Install Go 1.23+**
2.  **Clone the repo**:
    ```bash
    git clone https://github.com/joaodalvi/oci-arm-provisioner.git
    cd oci-arm-provisioner
    ```
3.  **Build**:
    ```bash
    make build
    ```

## Style Guide

*   We follow standard **Go functionality** and formatting (`gofmt`).
*   Comments should be full sentences.
*   Log messages should be structured (use the `logger` package).

## License

By contributing, you agree that your contributions will be licensed under its **GPLv3 License**.
