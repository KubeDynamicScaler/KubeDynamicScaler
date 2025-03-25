# KubeDynamicScaler

<div align="center">

![KubeDynamicScaler Logo](docs/images/kubedynamicscaler-logo.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/KubeDynamicScaler/KubeDynamicScaler)](https://goreportcard.com/report/github.com/KubeDynamicScaler/KubeDynamicScaler)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Release](https://img.shields.io/github/release/KubeDynamicScaler/kubedynamicscaler.svg)](https://github.com/KubeDynamicScaler/kubedynamicscaler/releases/latest)

</div>

> ⚠️ **IMPORTANT DISCLAIMER**: This project is currently under active development and is not yet ready for production use. Features may be incomplete, and breaking changes can occur without notice. We recommend using it only for testing and development purposes until a stable release is announced.

KubeDynamicScaler is an open-source Kubernetes controller that revolutionizes how you manage deployment replicas in your clusters. It provides a flexible, dynamic, and automated way to control application scaling through global configurations and specific overrides, while maintaining safety through exclusion rules.

## 🌟 Key Features

- **Global Replica Management**: Define cluster-wide scaling policies
- **Selective Overrides**: Apply specific scaling rules to targeted deployments
- **HPA/KEDA Integration**: Seamlessly work with existing auto-scaling solutions
- **Flexible Scaling Modes**: 
  - Global Mode: Defines a percentage for a global adjustment of replicas (can be to increase or to reduce)
  - Override Mode: Handle percentual override to workloads, selector or namespaces specified
  - Additive Mode: Stack percentages to be added to the overall percentage
  - Ignore Mode: Handle exceptions to workloads, selector or namespaces specified
- **Real-time Updates**: Configuration changes through ConfigMaps without restarts
- **Prometheus Metrics**: Built-in monitoring and alerting support
- **Kubernetes Native**: Follows Kubernetes patterns and best practices

## 🎯 Why KubeDynamicScaler?

KubeDynamicScaler addresses common challenges in Kubernetes deployments:

- **Cost Optimization**: Automatically adjust replicas based on global policies
- **Scheduled Events**: Prepares (warms up) the workload for scheduled events such as Black Friday, Cyber ​​Monday, Marketing Campaigns, etc.
- **Resource Efficiency**: Fine-tune scaling based on actual needs
- **Enterprise Ready**: Production-tested with monitoring and safety features to avoid the famous scripts with cronjobs for massive changes

## 🚀 Quick Start

### Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl
- Helm v3 (optional)

### Installation

#### Using Helm (Recommended)

```bash
helm repo add kubedynamicscaler https://kubedynamicscaler.github.io/charts
helm repo update
helm install kubedynamicscaler kubedynamicscaler/kubedynamicscaler -n kubedynamicscaler-system --create-namespace
```

#### Using kubectl

```bash
kubectl apply -f https://raw.githubusercontent.com/KubeDynamicScaler/kubedynamicscaler/main/deploy/manifests.yaml
```

### Basic Usage

1. Create a global configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubedynamicscaler-config
  namespace: kubedynamicscaler-system
data:
  config.yaml: |
    globalPercentage: 100
    maxReplicas: 100
    minReplicas: 1
```

2. Create an override for specific deployments:

```yaml
apiVersion: dynamicscaling.k8s.io/v1
kind: ReplicasOverride
metadata:
  name: high-load-override
  namespace: production
spec:
  selector:
    matchLabels:
      tier: frontend
  overrideType: "override"
  replicasPercentage: 150
```

## 📚 Documentation

Visit our [official documentation](https://kubedynamicscaler.io/docs) for:

- [Architecture Overview](https://kubedynamicscaler.io/docs/architecture)
- [Installation Guide](https://kubedynamicscaler.io/docs/installation)
- [Configuration Reference](https://kubedynamicscaler.io/docs/configuration)
- [Best Practices](https://kubedynamicscaler.io/docs/best-practices)
- [Troubleshooting](https://kubedynamicscaler.io/docs/troubleshooting)

## 🤝 Contributing

We love your input! We want to make contributing to KubeDynamicScaler as easy and transparent as possible. Check out our [Contributing Guide](CONTRIBUTING.md) to get started.

Ways you can contribute:
- Report bugs
- Suggest new features
- Submit pull requests
- Improve documentation
- Share your success stories

## 📅 Roadmap

See our [GitHub Project Board](https://github.com/KubeDynamicScaler/kubedynamicscaler/projects/1) for planned features and enhancements.

Upcoming features:

- [ ] Custom scaling algorithms

## 📜 License

KubeDynamicScaler is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🌟 Acknowledgments

Special thanks to:

- All our contributors
- Companies using and supporting KubeDynamicScaler

## 📫 Community & Support

- [GitHub Discussions](https://github.com/KubeDynamicScaler/kubedynamicscaler/discussions)

