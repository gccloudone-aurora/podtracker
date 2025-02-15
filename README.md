[(Français)](#le-nom-du-projet)

## PodTracker

A network auditing tool to help track pod IP allocations on a Kubernetes cluster

This project follows the [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

### How to Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md)

### License

Unless otherwise noted, the source code of this project is covered under Crown Copyright, Government of Canada, and is distributed under the [MIT License](LICENSE).

The Canada wordmark and related graphics associated with this distribution are protected under trademark law and copyright law. No permission is granted to use them outside the parameters of the Government of Canada's corporate identity program. For more information, see [Federal identity requirements](https://www.canada.ca/en/treasury-board-secretariat/topics/government-communications/federal-identity-requirements.html).

### Running Locally

When contributing new features, bug fixes, or any other functional change to the project, it is generally a good idea to (at the very least) run the solution against a Kubernetes cluster to validate the changes. Another common reason to run locally is simply to evaluate the solution as it applies to your use-case.

There are a few important steps to setting up an environment for running on a local cluster. In order to have a local cluster, you can use [KIND](https://kind.sigs.k8s.io/).

### Prerequisites

To run locally using KIND, you'll need at least:

- go 1.16+
- docker v24+ (use of `podman` requires extra configuration/setup not covered by this document)
  - ensure that your user is added to the `docker` group to be able to interface with the container runtime
  - use the [official docs](https://docs.docker.com/engine/install/) for installing docker

### Configuring the local cluster

Install the [KIND](https://kind.sigs.k8s.io/) CLI

```bash
go install sigs.k8s.io/kind@v0.20.0
```

Create a local cluster

```bash
kind create cluster --name local
```
> Note: If cluster creation fails, try running with `--retain` and use `docker logs <container_id>` to determine where the creation is failing
>
> If cluster creation fails on WSL and mentions an issue with mounting `/sys/fs/cgroup/systemd` try [this workaround](#running-locally-with-kind)

To avoid accidentally running on a real cluster, ensure that you have your KIND cluster selected as your current kube-context

```bash
kubectl config current-context
```

You should see a name that looks something like "`kind-local`" where `local` is the name you provided with `kind create cluster`.

**Optionally** Install the [cert-manager](https://cert-manager.io/) and [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) - these components are used for operator metrics and automatic webhook certificate management. For more information, see the [kubebuilder docs (webhook)](https://book.kubebuilder.io/cronjob-tutorial/running-webhook) or [kubebuilder docs (cert-manager)](https://book.kubebuilder.io/cronjob-tutorial/cert-manager).

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.3/cert-manager.yaml
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack
```
> **Note** this only installs a minimal configuration and may not be suitable for production

### Building the controller-manager image (podtracker)

Once you have a local (or remote) cluster set up, you will likely encounter a situation where you want to test our code without building and publishing the podtracker image to a remote container repository. For this, KIND provides a simple workflow that we can leverage for local development.

First, we build the image using docker

```bash
# from the project directory (that contains the Dockerfile)
docker build -t podtracker:dev .
```
> optionally set `--build-arg IMAGE_REPOSITORY=docker.io` to override the image repository used to build the container image

Then, with KIND, load the image onto the KIND Nodes using the following command

```bash
kind load docker-image podtracker:dev --name local
```

Before deploying, we should ensure that all the generated deepcopy implementations and manifests are current

```bash
make generate && make manifests
```
> **Note** generally, this is only required if you've modified components that are exposed as a Kubernetes API/object. In this project, this is typically any type under `api/vX/*`.

Deploy the solution (default configuration)

```bash
make deploy IMG=podtracker:dev
```
> By defining `IMG`, we tell kustomize to override the controller image name and tag so that your local KIND cluster knows to use the image we loaded to the cluster instead of the default controller image

### Deploying with extras

The solution can be deployed in the following configurations:

- `default`: only deploys the controller and CRDs (no metrics support, mutating webhooks, validating webhooks)
- `dev`: only deploys the controller, CRDs and metrics support (prometheus service monitor)
- `dev-webhooks`: deploys everything (controller, CRDs, metrics support (prometheus), mutating webhooks, validating webhooks (including Certificate CRDs for cert-manager to inject certs for webhooks))

Once you've decided which configuration you wish to deploy

```bash
make deploy ENV=$ENV IMG=podtracker:dev
```
> where `$ENV` is either `default`, `dev`, or `dev-webhooks`
>
> **Note** `default` will be used if no `ENV` is specified

### Configure a PodTracker

The PodTracker operator is configured through its accompanying CRDs (namely, the `PodTracker` CRD). An example configuration would look something like this

```yaml
apiVersion: networking.aurora.gc.ca/v1
kind: PodTracker
metadata:
  name: podtracker-example
spec:
  nsToWatch:
  - '*-system'
  backendWriterConfig:
    stdout:
      enabled: true
```

### See PodTracker in Action!

There are many ways you could test PodTracker. Assuming you deployed PodTracker using the above instructions and have also applied the PodTracker CR in the previous step, a simple use-case would be to simply create a Pod, watch PodTracker react, delete the Pod, and watch PodTracker react again.

Create a Pod

```bash
cat <<EOF | kubectl apply -f-
apiVersion: v1
kind: Pod
metadata:
  name: sample-pod
  namespace: kube-system
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
EOF
```

Take a look at the controller logs

```bash
kubectl logs -n podtracker-system <podtracker_pod_name>
```
> This implies use of the default stdout backend writer

You should see something like this

```
{
  "trackedBy": "podtracker-example",
  "id": "7a4f2fa3-e4b5-44ae-a7cb-426bbacc31a0",
  "event": "Create",
  "name": "sample-pod",
  "namespace": "kube-system",
  "labels": null,
  "annotations": {},
  "creationTimestamp": "2024-01-24T20:44:08+0000",
  "deletionTimestamp": "",
  "podIP": "10.244.0.17",
  "node": "local-control-plane",
  "nodeIPs": {
    "Hostname": [
      "local-control-plane"
    ],
    "InternalIP": [
      "172.18.0.2"
    ]
  }
}
```

Now we can delete the Pod

```bash
kubectl delete pod -n kube-system sample-pod
```

Take a look at the controller logs

```bash
kubectl logs -n podtracker-system <podtracker_pod_name>
```
> This implies use of the default stdout backend writer

You should see something like this

```
{
  "trackedBy": "podtracker-example",
  "id": "7a4f2fa3-e4b5-44ae-a7cb-426bbacc31a0",
  "event": "Delete",
  "name": "sample-pod",
  "namespace": "kube-system",
  "labels": null,
  "annotations": {},
  "creationTimestamp": "2024-01-24T20:44:08+0000",
  "deletionTimestamp": "2024-01-24T20:48:07+0000",
  "podIP": "10.244.0.17",
  "node": "local-control-plane",
  "nodeIPs": {
    "Hostname": [
      "local-control-plane"
    ],
    "InternalIP": [
      "172.18.0.2"
    ]
  }
}
```
> Notice the `deletionTimestamp` field has now been populated!
>
> The remaining details should generally be the same

### Cleanup

Working with KinD has many advantages. One of which is that it's incredibly simple to spin up and tear down. To clean up your environment, simply delete the KinD cluster

```bash
kind delete cluster -n local
```

### Installation and Usage

Install using [Helm](/install/kubernetes/podtracker/)

## Troubleshooting

### Running Locally (with KinD)

#### Cluster creation fails at mounting `/sys/fs/cgroup/systemd` on WSL

Kind nodes require systemd cgroup mount  which doesn't always exist on the WSL host. As a result, we can workaround the issue by creating the mountpoint as shown:

```bash
sudo mkdir /sys/fs/cgroup/systemd
sudo mount -t cgroup -o none,name=systemd cgroup /sys/fs/cgroup/systemd
```

Deploy a [`PodTracker`](/config/samples/networking_v1_podtracker.yaml) configuration.

______________________

## PodTracker

__TODO__

### Comment contribuer

Voir [CONTRIBUTING.md](CONTRIBUTING.md)

### Licence

Sauf indication contraire, le code source de ce projet est protégé par le droit d'auteur de la Couronne du gouvernement du Canada et distribué sous la [licence MIT](LICENSE).

Le mot-symbole « Canada » et les éléments graphiques connexes liés à cette distribution sont protégés en vertu des lois portant sur les marques de commerce et le droit d'auteur. Aucune autorisation n'est accordée pour leur utilisation à l'extérieur des paramètres du programme de coordination de l'image de marque du gouvernement du Canada. Pour obtenir davantage de renseignements à ce sujet, veuillez consulter les [Exigences pour l'image de marque](https://www.canada.ca/fr/secretariat-conseil-tresor/sujets/communications-gouvernementales/exigences-image-marque.html).
