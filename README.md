# Shawarma Webhook

[![ci](https://github.com/CenterEdge/shawarma-webhook/actions/workflows/docker-image.yml/badge.svg)](https://github.com/CenterEdge/shawarma-webhook/actions/workflows/docker-image.yml)

A Kubernetes Mutating Admision Webhook which will automatically apply the Shawarma sidecar when requested via annotations.

## Deploying

The webhook is typically deployed to the kube-system namespace. An example deployment can be
[found in the main Shawarma repository](https://github.com/CenterEdge/shawarma/tree/master/example/injected).

Note that the example assumes that [cert-manager](https://cert-manager.io/) has been installed on
your cluster to manage TLS between the API server and the webhook.

## RBAC Rights

If using `SHAWARMA_SERVICE_ACCT_NAME` (the default), the webhook needs the following RBAC rights bound to
the webhook's service account.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: shawarma-webhook
rules:
- apiGroups: [""]
  resources: ["serviceaccounts"]
  verbs: ["get", "watch", "list"]
```

## Environment Variables

The following environment variables may be used to customize behaviors of the webhook.

| Name                       | Default                              | Description |
| -------------------------- | ------------------------------------ | ----------- |
| LOG_LEVEL                  | warn                                 | Log level for the admission webhook |
| WEBHOOK_PORT               | 8443                                 | Port used by the admission webhook |
| CERT_FILE                  | /etc/shawarma-webhook/certs/tls.crt  | Certificate file used for TLS by the admission webhook |
| KEY_FILE                   | /etc/shawarma-webhook/certs/tls.key  | Key file used for TLS by the admission webhook |
| SWAWARMA_IMAGE             | centeredge/shawarma:1.0.0            | Default Shawarma image |
| SHAWARMA_SERVICE_ACCT_NAME | shawarma                             | Name of the service account which should be used for sidecars |
| SHAWARMA_SECRET_TOKEN_NAME |                                      | Name of the secret containing the Kubernetes token for Shawarma, overrides SHAWARMA_SERVICE_ACCT_NAME |

## Annotations

The following annotations may be applied to alter behaviors on a specific pod.

| Name                                    | Required         | Description |
| --------------------------------------- | ---------------- | ----------- |
| `shawarma.centeredge.io/service-name`   | Y (if no labels) | Name of the K8S service to be monitored, the sidecar is not injected if this annotation is not present |
| `shawarma.centeredge.io/service-labels` | Y (if no name)   | K8S service labels to monitor, comma-delimited ex. `label1=value1,label2=value2` |
| `shawarma.centeredge.io/image`          | N                | Override the image used for Shawarma |
| `shawarma.centeredge.io/log-level`      | N                | Override the log level used by Shawarma |
| `shawarma.centeredge.io/state-url`      | N                | Override the URL which receives Shawarma application state (default `http://localhost/applicationstate`) |
| `shawarma.centeredge.io/listen-port`    | N                | Override the port on which the Shawarma sidecar listens for state requests, (default `8099`) |

## Customizing The Sidecar

The sidecar is configured via the `./sidecar.yaml` file which is included in the Docker image. It may
add volumes and containers to pods which have the Shawarma annotations.

This file may be replaced with a custom version using a volume mount. The `--config /path/to/sidecar.yaml`
command line argument configures the location of the custom file. This can be used to change the resource
allocations or other details of the sidecar.

| Replacement Token     | Description |
| -----------------     | ----------- |
| `SHAWARMA_IMAGE`      | Must be in a container `image`, replaced with the configured Shawarma image |
| `SHAWARMA_TOKEN_NAME` | Must be in a volume `secretName`, replaced with the name of the secret containing the Shawarma token for K8S API access |

> For an example SIDECAR_CONFIG file, see [sidecar.yaml](./sidecar.yaml).
