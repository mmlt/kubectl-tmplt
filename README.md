# kubectl-tmplt

A `kubectl` plugin to expand and apply templated resource files to a cluster.

Compared to Helm it;
- doesn't use 'charts' as intermediate when applying resources to target cluster.
- allows to wait for certain conditions in the target cluster before (continuing) applying resources.


## Quick Start

Install:
```
kubectl krew install kubectl-tmplt # or download manually.
kubectl tmplt --help
```

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


More examples in `test/e2e/testdata/`


## Wishlist
- set-value flag
- add label (also needed for prune)
- prune
- kustomize step