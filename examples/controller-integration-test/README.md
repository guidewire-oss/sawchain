# Example: Controller Integration Test

This example demonstrates how Sawchain can be used to test K8s controllers and webhooks

* Controller tests: [controllers/podset_controller_test.go](./controllers/podset_controller_test.go)
* Webhook tests: [api/v1/podset_webhooks_test.go](./api/v1/podset_webhooks_test.go)

## How To Run

```sh
make init generate test
```

## References

* [What is a controller?](https://book-v1.book.kubebuilder.io/basics/what_is_a_controller)
* [What is a webhook?](https://book-v1.book.kubebuilder.io/beyond_basics/what_is_a_webhook)
* [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)
