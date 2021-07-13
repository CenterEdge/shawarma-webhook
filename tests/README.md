# shawarma-webhook tests

These tests help confirm functionality of the webhook within Docker. It is primarily intended
for use with a local Docker Desktop installation of Kubernetes.

## Using

```sh
kubectl apply -f ./cert-manager.yaml # Installs the cert-manager in the cert-manager namespace
kubectl apply -f ./common.yaml # Installs various common resources

# Build a local image to test
docker build -t centeredge/shawarma:test-001 --build-arg VERSION=1.0.0 ../

# Edit webhook-deployment.yaml to reference the image built above
kubectl apply -f ./webhook-deployment.yaml # Runs the webhook to be tested

# Start a test service to see it receive Shawarma containers
kubectl apply -f ./test-service.yaml
kubectl get pods -n shawarma-test
```
