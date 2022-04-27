# Crossplane Operator DFDS
A Kubernetes [operator](https://sdk.operatorframework.io/) to support the Crossplane implementation in DFDS.

It works by detecting an annotation on the namespace and if set to true it will install the DFDS Crossplane pre-requisites such as RBAC rules for multi-tenancy and ProviderConfig into the namespace using an AWS Account ID annotation on the namespace.

## Configuration

### Environment variables

The operator can be configured with the following environment variables:

`DFDS_CROSSPLANE_ENABLED_ANNOTATION_NAME` - This variable is used to set the annotation name for the operator to obtain whether Crossplane should be enabled on the namespace.

`DFDS_CROSSPLANE_AWS_ACCOUNT_ID_ANNOTATION_NAME` - This variable is used to set the annotation name for the operator to obtain the AWS account ID from

`DFDS_CROSSPLANE_PKG_ALLOWED_API_GROUPS` - This varialbe is used to configure the allowed API groups to apply to the RBAC when enabling Crossplane

## Installing

### Helm chart

The operator can be installed via helm using the following command:

```
helm install -n crossplane-system -f values.yaml crossplane-operator-dfds dfds/crossplane-operator-dfds
```

Environment variables can be set in the values.yaml file you provide to helm as follows:

```
crossplaneOperatorDfds:
  enableAnnotionName: "dfds-crossplane-enabled"
  awsAccountAnnotationName: "dfds-aws-account-id"
```

### Locally
Useful for development/testing - With your appropriate kube context set you can run the operator locally by cloning the repository and running the following command, replacing values for the environment variables as required:

```
DFDS_CROSSPLANE_PKG_ALLOWED_API_GROUPS="storage.xplane.dfds.cloud,certs.xplane.dfds.cloud,cdn.xplane.dfds.cloud, compute.xplane.dfds.cloud" DFDS_CROSSPLANE_ENABLED_ANNOTATION_NAME="dfds-crossplane-enabled" DFDS_CROSSPLANE_AWS_ACCOUNT_ID_ANNOTATION_NAME="dfds-aws-account-id" make run
```

### Building

The application can be built using the following command:

```
make build
```

To build a docker image update the `IMAGE_TAG_BASE` and `VERSION` in the makefile and run:

```
make docker build
```

To push the docker image to a repository, run:

```
make docker push
```