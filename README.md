# kubectl-tmplt

A `kubectl` plugin to expand and apply templated resource files to a cluster.

Compared to Helm it;
- doesn't use 'charts' as intermediate when applying resources to target cluster.
- allows to wait for certain conditions in the target cluster before (continuing) applying resources.


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
- [could have] 'delete' step. Use-case: delete CR and wait for operator to do it's work, then delete(prune?) operator. 
- [could have] prune stores a list of deployed resources instead of querying the cluster by labels.


## Known issues

### Prune
Prune is slow. Prune queries the cluster for all `kubectl api-resources` in the prune.namespaces and this takes one or more minutes.

Prune expects the group of an object to stay the same. For example deploying Ingress with `apiVersion: networking.k8s.io/v1beta1`
results in an Ingress `apiVersion: extensions/v1beta1` and causes it to be pruned.
In such a case update the group in the templates.


  