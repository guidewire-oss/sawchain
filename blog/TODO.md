# TODO

* Expand upon problems more:
  * Lack of effective standard tools leads to each project implementing its own quirky test helpers
  * Representing YAML entities with structs is verbose, tedious, and unnatural, and often structs are not easily available for 3rd-party APIs
  * YAML-driven tests would be a much better representation of the actual user experience (aligns with BDD)
  * Naive assertions often produce vague error messages (e.g. "wait timeout exceeded") or specific values without context (e.g. "expected false to be true") when the fail, and failure outputs are hard to standardize without common tooling
* Explain how Sawchain behaves when given just objects, and how it behaves when objects are given with templates.
  * With just objects, objects are used to read and write state.
  * With objects and templates, templates are used to write, and state is written back to the objects (so the state can be used later in the test).
* Add more of an explanation for the different types of tests:
  * Offline render tests: asserting on YAML outputs of compositions or templating logic (e.g. Helm charts, Crossplane compositions, KubeVela components)
  * Integration tests (controllers, webhooks, client operations)
* Demonstrate how Sawchain improves and streamlines test failure outputs using Chainsaw's field errors
  * It provides a clear YAML diff between the actual state and the expected state, listing each specific field that failed to match
  * Edge cases like no candidates found have clear error messages as well
  * Include examples of output (these may need to be manually modified)
