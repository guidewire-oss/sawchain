# Example: `helm install` Test

This example demonstrates how Sawchain can be used to test the install and upgrade behavior of [Helm](https://helm.sh/) charts

## How To Run

1. Ensure you have the [Helm CLI](https://helm.sh/docs/intro/install/) installed
1. Create or log into a K8s cluster (e.g. `k3d cluster create example`)
1. Run `go test -v`

## References

* [Using Helm](https://helm.sh/docs/intro/using_helm/)
* [nginx](https://nginx.org/en/)
