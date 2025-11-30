# node-life-support

Emergency life-support for Kubernetes nodes.

For when you have kubelets that cannot communicate with their control-plane, but
you know the nodes are healthy and want to keep them and their service endpoints
online.

Ordinarily, if a kubelet cannot reach or authenticate to kube-apiserver, it will
fail to renew its node lease and fail to update its node resource. The node will
then become NotReady and service endpoints will be removed.

This small controller periodically renews the node lease and patches node status
on behalf of the nodes, so that they remain Ready.

This project may be of particular interest to those who run clusters with
remote control-planes, such as AWS EKS clusters extended into AWS Outposts.

## Deployment

### Using raw manifests:

```bash
kubectl apply -f manifests/
```

### Using the Helm chart:

```bash
helm install node-life-support chart/node-life-support --namespace node-life-support --create-namespace
```

## Configuration

Environment variables used by the controller:

`NODE_LABEL_ALLOWLIST` - comma-separated list of node label keys. Only nodes with at least one of these labels will be put on life support.
If this is not set, all nodes in the cluster will be put on life support.

## Building

1. Build the binary (requires Go >=1.22):

```bash
go build -o bin/node-life-support ./
```

2. Run unit tests:

```bash
go test ./...
```

3. Build container image:

```
docker buildx build --platform linux/amd64,linux/arm64 .
```

## Contributing

Please read `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md` before opening issues or PRs.

## License

This project is licensed under the MIT License — see the `LICENSE` file.
