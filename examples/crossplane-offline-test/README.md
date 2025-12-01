# Example: Crossplane Offline Verification Test

This example demonstrates how Sawchain can be used to test rendering and schema validation of
Crossplane v2 function-based [compositions](https://docs.crossplane.io/latest/concepts/compositions/)
offline (without a K8s cluster)

## How To Run

1. Ensure you have Docker running and the [Crossplane v2 CLI](https://docs.crossplane.io/latest/cli/) installed
1. Run `ginkgo -v` or `go test -v`

## Composition Details

The example composition ([yaml/composition.yaml](./yaml/composition.yaml)) provisions an AWS IAM user
with an optional access key and optional policy attachments.

### Pipeline Steps

0. **load-environment-config**: Uses `function-environment-configs` to load environment configuration from a statically named EnvironmentConfig resource
1. **create-iam-user**: Uses `function-go-templating` to render a User managed resource (MR)
2. **create-access-key**: Uses `function-go-templating` to conditionally render an AccessKey MR (only if the XR specifies `spec.createAccessKey=true` and
   the composed User's ID is available) and writes a connection secret in the XR's namespace
3. **attach-policies**: Uses `function-go-templating` to conditionally render multiple UserPolicyAttachment MRs by looping over the XR's `spec.policyArns` (only if the composed User's ID is available)
4. **propagate-status**: Uses `function-go-templating` to copy important status properties (ARNs, IDs, etc.) from the composed resources to the XR's status
5. **auto-ready**: Uses `function-auto-ready` to automatically set the XR's status conditions based on the readiness of the composed resources

## References

* [function-environment-configs](https://github.com/crossplane-contrib/function-environment-configs)
* [function-go-templating](https://github.com/crossplane-contrib/function-go-templating)
* [function-auto-ready](https://github.com/crossplane-contrib/function-auto-ready)
* [provider-aws-iam](https://marketplace.upbound.io/providers/upbound/provider-aws-iam)
