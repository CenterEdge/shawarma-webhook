# Shawarma Webhook

[![ci](https://github.com/CenterEdge/shawarma-webhook/actions/workflows/docker-image.yml/badge.svg)](https://github.com/CenterEdge/shawarma-webhook/actions/workflows/docker-image.yml)

A Kubernetes Mutating Admision Webhook which will automatically apply the Shawarma sidecar when requested via annotations.

## Deploying

The webhook is typically deployed to the kube-system namespace. An example deployment can be
[found in the main Shawarma repository](https://github.com/CenterEdge/shawarma/tree/master/example/injected).

Note that the sample uses predefined, publicly available certificates to secure communication between
the Kubernetes API Server and the webhook. For production, these certificates should be replaced with
secure, locally generated certificates.

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
| WEBHOOK_PORT               | 443                                  | Port used by the admission webhook |
| CERT_FILE                  | /etc/shawarma-webhook/certs/cert.pem | Certificate file used for TLS by the admission webhook |
| KEY_FILE                   | /etc/shawarma-webhook/certs/key.pem  | Key file used for TLS by the admission webhook |
| SWAWARMA_IMAGE             | centeredge/shawarma:0.1.2            | Default Shawarma image |
| SHAWARMA_SERVICE_ACCT_NAME | shawarma                             | Name of the service account which should be used for sidecars |
| SHAWARMA_SECRET_TOKEN_NAME |                                      | Name of the secret containing the Kubernetes token for Shawarma, overrides SHAWARMA_SERVICE_ACCT_NAME |

## Annotations

The following annotations may be applied to alter behaviors on a specific pod.

| Name                                  | Required | Description |
| ------------------------------------- | -------- | ----------- |
| `shawarma.centeredge.io/service-name` | Y        | Name of the K8S service to be monitored, the sidecar is not injected if this annotation is not present |
| `shawarma.centeredge.io/image`        | N        | Override the image used for Shawarma |
| `shawarma.centeredge.io/log-level`    | N        | Override the log level used by Shawarma |
| `shawarma.centeredge.io/state-url`    | N        | Override the URL which receives Shawarma application state (default `http://localhost/applicationstate`) |
