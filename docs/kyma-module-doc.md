# Kyma community module

This operator is also available on Kyma as a community module. You can install it via the Kyma dashboard and it will install everything automatically and deploy all Cloud Active Defense dependencies.

## Update the module

If you want to update the community module in Kyma, there are a few steps to follow:

### 1. Make a release

Once you have pushed all your changes in the repo and the operator is stable, you can make a release on GitHub so the image of the operator is rebuilt with your changes and publicly available. By publishing a release the GitHub workflow will automatically generate the manifest (required to install the operator into k8s) and attach it to the release.

### 2. Edit the default Custom Resource (optional)

If in your operator changes you added a new property that is mandatory or not set by default in the code, for the `Cloud Active Defense` Custom Resource, you must set it in the `default-cad-CR.yaml` file.

### 3. Update the community module

First you must fork the [Kyma community module repo](https://github.com/kyma-project/community-modules).
Then go to the `/modules/cad-operator` directory and edit the existing file (`cad-operator-x.y.z.yaml`).

Steps to edit the module file:
- Rename the file to match the release version you published
- Change the version everywhere in the file (`metadata.name`, `spec.version` and `spec.data.metadata.labels.app.app.kubernetes.io/version`)
- **(Optional)** Adapt the Custom Resource properties in `spec.data.spec` with what you set in the default Custom Resource (see [Edit the default Custom Resource](#2-edit-the-default-custom-resource-optional))
- ⚠️ **Most important step:** Edit the `rawManifest` link with the new manifest auto-generated in the new release. You can just change the version in the link to the new release one or copy the manifest link from the release assets

---

When you are done, you can go back to the original Kyma community module repo, create a Pull request and select your forked repo.

When the Pull request is accepted you should see the new version of the Cloud Active Defense Operator module in the Kyma dashboard after a few minutes.