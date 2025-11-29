# Example: KubeVela Integration Test

This example demonstrates how Sawchain can be used to test [KubeVela](https://kubevela.io/docs/) components and traits

## How To Run

1. Create or log into a K8s cluster (e.g., `k3d cluster create example`)
1. Ensure KubeVela is installed (e.g., `vela install`)
1. Run `ginkgo -v` or `go test -v`

## References

* [KubeVela webservice component](https://kubevela.io/docs/end-user/components/references/#webservice)
* [KubeVela gateway trait](https://kubevela.io/docs/end-user/traits/references/#gateway)
* [KubeVela CLI](https://kubevela.io/docs/installation/kubernetes/)
