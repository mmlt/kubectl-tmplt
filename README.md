# kubectl-tmplt

A `kubectl` plugin to expand and apply templated resource files to a cluster.

Features;
- doesn't use 'charts' as intermediate when applying resources to target cluster.
- allows to wait for certain conditions in the target cluster before (continuing) applying resources.
- can fetch values from a "master vault" for use in templating or actions.
- can prune.
- can label all resources.
- can perform actions to:
  - read value from cluster to use in subsequent templating steps.
  - write to HashiVault policy or logical paths 


## Quick Start

Download `kubectl-tmplt` from [releases](https://github.com/mmlt/kubectl-tmplt/releases) and put it in your PATH.
Check `kubectl tmplt --help`


Create a 'job.yaml' file:
```yaml
steps:
- tmplt: "template.yaml"
  values:
    name: "test"
- wait: --for condition=Ready pod -l app=example

defaults:
  namespace: "default"
```

Create a `template.yaml` file:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "{{ .Values.name }}"
  namespace: "{{ .Values.namespace }}"
  labels:
    app: example
spec:
  containers:
    - args: [sleep, "3600"]
      image: ubuntu
      name: ubuntu
```

Create a `values.yaml` file with value overrides:
```yaml
#values:
#  name: optionally override values defined in template
```

See what is going to be applied; 
`kubectl tmplt -m generate --job-file job.yaml -set-file values.yaml`


Before continuing make sure `kubectl config current-context` selects the target cluster.

Apply;
`kubectl tmplt -m apply --job-file job.yaml -set-file values.yaml`

Clean-up;
`kubectl tmplt -m generate --job-file all.yaml -set-file values.yaml | kubectl delete -f -`


More examples in `test/e2e/testdata/` and `kubectl tmplt --help`


## Wishlist
- Support templates with other delimiters than {{ }}. Use-case; prometheus config uses {{ }} but needs to be templated as well.
  Possible delimters: <% %> {~ ~} (or configurable via tmplt step parameter)
- [could have] 'delete' step. Use-case: delete CR and wait for operator to do it's work, then delete(prune?) operator.
- Support annotation deploy.mmlt.nl/create=delete|recreate to specify how immutable objects should be handled.
  For each object with this annotation kubectl-tmplt will get the object from the cluster and if `.spec` differs will
  either deletes/create the object or recreate the object.
  If the object has no annotation or `.spec` are equal kubectl-tmplt will apply the object.   


## Known issues





  