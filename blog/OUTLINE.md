# Kubernetes Deserves Better Tests: Meet Sawchain

## The Problem — K8s Testing is Hard

* Tests are too imperative
* Writing them is verbose and repetitive
* Flaky behavior due to race conditions (e.g., waiting for readiness)
* YAML lives separately from test code
* Assertion logic is custom and hard to standardize

## What We Wanted (But Couldn’t Find)

Ideals:

* Clean YAML assertions where they make sense (75% of fields)
* Flexibility to use structs and Gomega matchers when needed (25% of fields)
* Easy and reliable polling functions for async updates and client caching
* Assertion tools that are reusable for any type of test (offline render tests, integration, etc.)
* Less boilerplate, better readability
* Works great in CI

Existing tools and their limitations:

kuttl
chainsaw
helm-unittest

## Introducing Sawchain

Include:

* Link to the GitHub repo (open-source: <https://github.com/guidewire-oss/sawchain>)
* A short example block of code (before vs after using Sawchain)

## Examples

Extract snippets and explanations from Sawchain project examples folder
Demonstrate how Sawchain addresses the problems and ideals listed earlier in the post

## Try It

Call to action
