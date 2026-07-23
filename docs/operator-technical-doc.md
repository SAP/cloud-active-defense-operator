# Cloud Active Defense Operator

## Requirements
- Docker
- Kubectl
- Kubectl kustomize

## Controller
The controller is the brain of the operator that automates the deployment and keeps it running correctly.

A "Reconcile" function watches for a `CloudActiveDefense` custom resource creation or update and deploys Cloud Active Defense deployment, secrets, services...

The file handling the controller code is located at `internal/controller/cloudactivedefense_controller.go`

Also in `internal/controller/resources.go` you can find all of the definition and structure of the deployment, secrets... 

## Build

After changes in the code some commands must be ran in order to update related config files that are automatically generated.

This will generate Kubernetes config files for your CRDs (CustomResourceDefinitions) and RBAC config from the `/api/v1alpha1/cloudactivedefense_types.go` file
```sh
make manifests
```
The results of this command are YAML files located in `config/`. Since this is auto generated files, nothing have to be change here.

This will generate Go code from the `/api/v1alpha1/cloudactivedefense_types.go` file so the Kubernetes API can work with operator Go types
```sh
make generate
```

## Docker

You can build the Docker image of the operator by running this command
```sh
docker build -t controller .
```

This Docker image corresponds to the operator controller (the Go code handling our custom resource). The controller will watch for new `CloudActiveDefense` custom resources created in the cluster, then it will automatically deploy all necessary resources needed to run Cloud Active Defense (Controlpanel, deployment manager...).

This Docker image of the controller is also hosted in the GitHub repo: `ghcr.io/sap/cad-controller:latest`

## Manifests

The manifest is a configuration file that combines all generated files of the `make manifests` command into one single file to deploy the operator to the cluster.

To generate the manifests that will help you to deploy the operator on your cluster, you can run this
```sh
kustomize build config/default > manifest.yaml
```

This manifest file is also attached to the GitHub repository in the latest release: https://github.com/SAP/cloud-active-defense-operator/releases

To deploy the operator to your cluster you can use kubectl with the manifests.yaml you generated or the one hosted on GitHub
```sh
kubectl apply -f manifests.yaml
```

## Deploy Cloud Active Defense

Once the manifest is deployed, the controller is waiting for a `Cloud Active Defense` custom resource. You can create your own or use the default one in the GitHub repo (https://raw.githubusercontent.com/SAP/cloud-active-defense-operator/refs/heads/main/default-cad-CR.yaml) and apply it with kubectl
```sh
kubectl apply -f default-cad-CR.yaml
```

Then Cloud Active Defense will automatically install all its dependencies.
