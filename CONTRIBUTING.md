# Contributing to KubeDynamicScaler

First off, thank you for considering contributing to KubeDynamicScaler! It's people like you that make KubeDynamicScaler such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by the [KubeDynamicScaler Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check [this list](https://github.com/KubeDynamicScaler/kubedynamicscaler/issues) as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

* **Use a clear and descriptive title**
* **Describe the exact steps which reproduce the problem**
* **Provide specific examples to demonstrate the steps**
* **Describe the behavior you observed after following the steps**
* **Explain which behavior you expected to see instead and why**
* **Include cluster details** (Kubernetes version, cloud provider if applicable)
* **Include KubeDynamicScaler version and configuration**
* **Include any relevant logs**

### Suggesting Enhancements

Enhancement suggestions are tracked as [GitHub issues](https://github.com/KubeDynamicScaler/kubedynamicscaler/issues). When creating an enhancement suggestion, please include:

* **Use a clear and descriptive title**
* **Provide a step-by-step description of the suggested enhancement**
* **Provide specific examples to demonstrate the steps**
* **Describe the current behavior and explain the behavior you expected to see instead**
* **Explain why this enhancement would be useful**
* **List some other tools or applications where this enhancement exists**

### Pull Requests

* Fork the repo and create your branch from `main`
* If you've added code that should be tested, add tests
* If you've changed APIs, update the documentation
* Ensure the test suite passes
* Make sure your code lints
* Issue that pull request!

## Development Setup

1. Install the prerequisites:
   * Go 1.21 or higher
   * Kubernetes cluster (1.24+)
   * kubectl
   * kubebuilder
   * kustomize

2. Clone the repository:
```bash
git clone https://github.com/KubeDynamicScaler/kubedynamicscaler.git
cd kubedynamicscaler
```

3. Install dependencies:
```bash
go mod download
```

4. Run the tests:
```bash
make test
```

5. Run the controller locally:
```bash
make run
```

## Project Structure

```
.
├── api/                    # API definitions
├── config/                 # Kubernetes manifests
├── controllers/            # Controller implementations
├── docs/                  # Documentation
├── hack/                  # Scripts and tools
├── pkg/                   # Shared packages
└── test/                  # Test files
```

## Coding Style

* Follow the standard Go project layout
* Use `gofmt` for formatting
* Follow [Effective Go](https://golang.org/doc/effective_go.html) principles
* Write meaningful commit messages following [Conventional Commits](https://www.conventionalcommits.org/)
* Include comments for exported functions and types
* Add unit tests for new functionality

## Testing

* Write unit tests for new code
* Include integration tests for new features
* Test edge cases and error conditions
* Use table-driven tests when appropriate
* Aim for high test coverage

## Documentation

* Update relevant documentation for any changes
* Document new features or behavior changes
* Include examples in documentation
* Keep the README.md up to date
* Add godoc comments for exported functions

## Review Process

1. Create a pull request with a clear title and description
2. Wait for the CI checks to pass
3. Address any review comments
4. Once approved, your PR will be merged
5. Celebrate your contribution! 🎉

## Release Process

1. Releases are created from the `main` branch
2. Version tags follow [Semantic Versioning](https://semver.org/)
3. Release notes are generated automatically
4. Documentation is updated for each release

## Community

* Join our [Slack channel](https://kubernetes.slack.com/messages/kubedynamicscaler)
* Participate in [GitHub Discussions](https://github.com/KubeDynamicScaler/kubedynamicscaler/discussions)
* Follow us on [Twitter](https://twitter.com/kubedynamicscaler)
* Attend our monthly community meetings

## Getting Help

* Check the [documentation](https://kubedynamicscaler.io/docs)
* Ask in the Slack channel
* Create a [GitHub Discussion](https://github.com/KubeDynamicScaler/kubedynamicscaler/discussions)
* Join our community meetings

## Recognition

Contributors are recognized in:
* The [CONTRIBUTORS](CONTRIBUTORS.md) file
* Release notes
* Our website's contributor page
* Social media shoutouts

Thank you for contributing to KubeDynamicScaler! 🙏 