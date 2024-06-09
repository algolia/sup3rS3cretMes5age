# Supersecretmessage Helm Chart

This repository contains the Supersecretmessage Helm chart for installing
and configuring Supersecretmessage on Kubernetes. This chart supports multiple use
cases of Supersecretmessage on Kubernetes depending on the values provided.

## Prerequisites

To use the charts here, [Helm](https://helm.sh/) and [Vault](https://www.vaultproject.io/) must be configured for your
Kubernetes cluster.

The versions required are:

* **Helm 3.6+**
* **Vault 1.10+**
* **Kubernetes 1.22+** - This is the earliest version of Kubernetes tested.
  It is possible that this chart works with earlier versions but it is
  untested.

> :warning: **Please note**: Setting up Kubernetes, Helm and Vault is outside the scope of
this README. Please refer to the [Kubernetes](https://kubernetes.io/docs/home/), [Helm](https://helm.sh/docs/intro/install/) and [Vault](https://developer.hashicorp.com/vault/tutorials/kubernetes/kubernetes-raft-deployment-guide) documentation. You can install the last one as a [Chart](https://developer.hashicorp.com/vault/docs/platform/k8s/helm).
