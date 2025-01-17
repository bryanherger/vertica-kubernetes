# Introduction

This guide explains how to setup an environment to develop and test the Vertica operator.

# Software Setup
Use of this repo obviously requires a working Kubernetes cluster.  In addition to that, we require the following software to be installed in order to run the integration tests:

- [go](https://golang.org/doc/install) (version 1.16.2)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) (version 1.20.1).  If you are using a real Kubernetes cluster this will already be installed.
- [helm](https://helm.sh/docs/intro/install/) (version 3.5.0)
- [kubectx](https://github.com/ahmetb/kubectx/releases/download/v0.9.1/kubectx) (version 0.9.1)
- [kubens](https://github.com/ahmetb/kubectx/releases/download/v0.9.1/kubens) (version 0.9.1)
- [golangci-lint](https://golangci-lint.run/usage/install/) (version 1.41.1)
- [krew](https://github.com/kubernetes-sigs/krew/releases/tag/v0.4.1) (version 0.4.1) $HOME/.krew/bin must be in your path
- [stern](https://github.com/stern/stern) (version 1.15.0)
- [kuttl](https://github.com/kudobuilder/kuttl/) (version 0.9.0)
- [changie](https://github.com/miniscruff/changie) (version 0.5.0)

# Kind Setup
[Kind](https://kind.sigs.k8s.io/) is a way to setup a multi-node Kubernetes cluster using Docker.  It mimics a multi-node setup by starting a separate container for each node.  The requirements for running Kind are quite low - it is possible to set this up on your own laptop.  This is the intended deployment to run the tests in an automated fashion.

We have a wrapper that you can use to setup Kind and create a cluster suitable for testing Vertica. The following command creates a cluster named `cluster1` that has one master node and one worker node. It takes only a few minutes to complete:  

```
scripts/kind.sh init cluster1
```

After it returns, change the context to use the cluster. The cluster has its own kubectl context named kind-cluster1:

```
kubectx kind-cluster1
```

Test the container out by checking the status of the nodes:

```
kubectl get nodes
```

After Kind is up, you need to configure it to run the integration tests.  The `setup-int-tests.sh` script encompasses all of the setup:

```
scripts/setup-int-tests.sh
```



# Kind Cleanup

After you are done with the cluster, you can delete it with our helper script. Substitute `cluster1` with the name of your cluster:

```
scripts/kind.sh term cluster1
```

If you forgot your name, run Kind directly to get the clusters installed:

```
$ PATH=$PATH:$HOME/go/bin
$ kind get clusters
cluster1
```

# Developer Workflow

## Make Changes

The structure of the repo is as follows:
- **docker-vertica/**: has the necessary files to build a container of the Vertica server.  The RPM package that we depend on to build the container has to be sourced separately and isn't included in this repo.
- **docker-operator/**: has the necessary files to build the container that holds the operator.
- **docker-webhook/**: has the necessary files to build the container that holds the webhook. 
- **scripts/**: contains scripts that were written for the repository.  Some are needed by the Makefile to run some targets.  While others, such as *upgrade-vertica.sh* automate some manual tasks.
- **api/**: defines the spec of the CRD
- **pkg/**: includes all of the packages that we wrote for the operator
- **cmd/**: contain source code for each of the executables
- **bin/**: contains the compiled or downloaded binaries that this repository depends on
- **config/**: generated files of all of the manifests that make up theoperator.  Of particular importance is *config/crd/bases/vertica.com_verticadbs.yaml*, which shows a sample spec for our CRD.
- **tests/**: has the test files for e2e and soak testing
- **changes/**: stores changelog for past releases and details about the changes for the next release
- **hack/**: includes a boilerplate file of a copyright that is included on the generated files
- **helm-charts/**: contains the helm charts that are built by this repository

## Add a changelog entry

The changelog file is generated by [Changie](https://github.com/miniscruff/changie).  It separates the changelog generation from commit history, so any PR that has a notable change should add new changie entries.

## Build and Push Vertica Container

We currently make use of a few containers:
- **docker-vertica/Dockerfile**: This container is the long-running container that runs the vertica daemon.
- **docker-operator/Dockerfile**: This is the container that runs the operator.
- **docker-webhook/Dockerfile**: This is the container that runs the webhook.

In order to run Vertica in Kubernetes, we need to package Vertica inside a container.  This container is then referenced in the YAML file when we install the helm chart.

Before you can build the *docker-vertica* container, **you need to get a vertica RPM since we do not include one in this repo**.  The RPM must be for Vertica version 10.1.1 or higher.  Once you have one, you put it in the `docker-vertica/packages` directory with the name `vertica-x86_64.RHEL6.latest.rpm`.

```
cp /dir/vertica-x86_64.RHEL6.latest.rpm docker-vertica/packages/
```

Run this make target to build the necessary containers:

```
make docker-build
```

By default, this creates containers that are stored in the local docker daemon. The tag is either `latest` or if running in a Kind environment it is `kind`.  You can control the container names by setting the following environment variables prior to running the make target.

- **OPERATOR_IMG**: Name of the image for the operator.
- **VERTICA_IMG**: Name of the image for vertica.
- **WEBHOOK_IMG**: Name of the image for the webhook.


If necessary these variables can include the url of the registry.  For example, `export OPERATOR_IMG=myrepo:5000/verticadb-operator:latest`

The vertica container can be built in two sizes -- minimal size removes the tensorflow package that saves 240MB.  The default is the full image size.  To build the minimal containe, invoke the make target like this:

```
make docker-build MINIMAL_VERTICA_IMG=YES
```

You need to make these containers available to the Kubernetes cluster.  You can push them with the following make target.

```
make docker-push
```

This command will honour the same environment variables of the image as listed above.

Due to the size of the vertica image, this step can take in excess of 10 minutes when run on a laptop.

If your image builds fail silently, confirm that there is enough disk space in your Docker repository to store the built images.

## Generate controller files

We use the operator-sdk framework for the operator.  It provides tools to generate code to avoid having to write the boilerplate code ourselves.  As such, depending on what you changed you may need to periodically regenerate files.  You can do that with this command:

```
make generate manifests
```

## Run Linter

We run two different linters:
1. **Helm lint**: This uses the chart verification test that is built into Helm.
2. **Go lint**: This uses a few linters that you can use with Go lang.

Both of these linters can be run with this command:

```
make lint
```

## Run Unit Tests

We have unit tests for both the helm chart and the Go operator.  

The unit tests for the helm chart are stored in `helm-charts/vertica/tests`. They use the [unittest plugin for helm](https://github.com/quintush/helm-unittest). Some samples that you can use to write your own tests can be found at [unittest github page](https://github.com/quintush/helm-unittest/tree/master/test/data/v3/basic).  [This document](https://github.com/quintush/helm-unittest/blob/master/DOCUMENT.md) describes the format for the tests.

Unit testing for the Go operator uses the Go testing infrastructure.  Some of the tests standup a mock Kubernetes control plane using envtest, and runs the operator against that.  As is standard with Go, the test files are included in package directories and end with `_test.go`.

The helm chart testing and Go lang unit tests can be run like this:

```
make run-unit-tests
```

## Run Operator

There are three ways to run the operator: 1) You can run the operator locally in your shell , 2) you can package it in a container and deploy it in Kubernetes as a deployment object, or 3) you can run it using the verticadb-operator helm chart.  The first method is quicker to get the running, but it isn't the way the operator will run in real Kubernetes environments.

1.  Locally

```
make install run
```

This will run the operator synchronously in your shell.  You can hit Ctrl+C to stop the operator.
**NB:** With this method, you can run only ad-hoc tests, not integration and e2e tests

2. Kubernetes Deployment

You can have the operator watch the entire cluster or a specific namespace. To do that you should set the environment variable **WATCH_NAMESPACE**. The operator will be deployed in that specific namespace and will be triggered only if the verticadb is deployed in that namespace. If you specify a not existing namespace you will get an error and the operator deployment will be interrupted.
Here are the commands:
```
WATCH_NAMESPACE=your_namespace
make docker-build deploy
```

This will run the operator as a deployment within Kubernetes.  You can run `make undeploy` to tear down the deployment.

3. Helm Chart release

This is the most convenient way to run the operator. But before creating a release, you need to generate the manifests helm will use to install the operator. For that, simply use the target **helm-create-resources** :
```
make helm-create-resources
```
Now you can install the verticadb-operator in a specific namespace by running the **helm install** command. Example:
```
helm install -n random release_name helm-charts/verticadb-operator
```
This will run the operator in a namespace called **random** and will use the default image:tag defined in **.Values.image.name**. You can use `--set image.name=<img:tag>` to specify the image name and tag.
To tear down the release with your operator, just run **helm uninstall** command. Example:
```
helm uninstall release_name -n random
```

## Run Integration and e2e Tests

The integration tests are run through Kubernetes itself.  We use kuttl as the testing framework.  The operator must be running **as a Kubernetes deployment**. You can push the operator to the Kind cluster by:
```
make docker-build-operator docker-push-operator
```

**NB:** Make sure that your operator Dockerfile is up to date before starting the tests.
You can use this make target to kick off the test:
```
make run-int-tests
```

You can also call `kubectl kuttl` from the command line if you want more control -- for instance running a single test or preventing cleanup when the test ends. For example, you can run a single e2e test by:

```
kubectl kuttl test --test <name-of-your-test>
```

## Run Soak Tests

The soak test will test the operator over a long interval.  It works by splitting the test into multiple iterations.  Each iteration will generate a random workload that is comprised of pod kills and scaling.  At the end of each iteration we will wait for everything to come up.  If successful, it will proceed to do another iteration.  It continues this for a set number of iterations or indefinitely.

The tests in an iteration are run through kuttl.  The random test generation is done by the kuttl-step-gen tool.

You can run this test with the following make target:

```
make run-soak-tests
```

## Help
```
make help
```

# Problem Determination

## Kubernetes Events

The operator will generate Kubernetes events for some key scenarios.  This can be a useful tool when trying to understand what the operator is doing.  You can use the following command to see the events that were generated for an operator.

```
$ kubectl describe vdb mydb

...<snip>...
Events:
  Type    Reason                   Age    From                Message
  ----    ------                   ----   ----                -------
  Normal  Installing               2m10s  verticadb-operator  Calling update_vertica to add the following pods as new hosts: mydb-sc1-0
  Normal  InstallSucceeded         2m6s   verticadb-operator  Successfully called update_vertica to add new hosts and it took 3.5882135s
  Normal  CreateDBStart            2m5s   verticadb-operator  Calling 'admintools -t create_db'
  Normal  CreateDBSucceeded        92s    verticadb-operator  Successfully created database with subcluster 'sc1'. It took 32.5709857s
  Normal  ClusterRestartStarted    36s    verticadb-operator  Calling 'admintools -t start_db' to restart the cluster
  Normal  ClusterRestartSucceeded  28s    verticadb-operator  Successfully called 'admintools -t start_db' and it took 8.8401312s
```

## Memory Profiling

You can use the memory profiler to see where the big allocations are occurring and to help detect any memory leaks.  The toolset is [Google's pprof](https://golang.org/pkg/net/http/pprof/).  By default it is off.  You can enable it by adding a parameter when you start the operator.  The following are instructions on how to enable it for an already deployed operator.

1. Add the flag to the running deployment.

```
kubectl edit deployment operator-controller-manager
```

Then locate where the arguments are passed to the manager and add the argument `--enable-profiler`.  The arguments will look like this:

```
      ...
      - args:
        - --health-probe-bind-address=:8081
        - --metrics-bind-address=127.0.0.1:8080
        - --leader-elect
        - --enable-profiler
        command:
        - /manager
      ...
```

2.  Wait until the operator is redeployed.
3.  Port forward 6060 to access the webUI for the profiler.  The name of the pod will differ for each deployment, so be sure to find the one specific to your cluster.

```
kubectl port-forward --address 0.0.0.0 pod/operator-controller-manager-5dd5b54df4-2krcr 6060:6060
```

4.  With a webbrowser or the standalone tool connect to `http://localhost:6060/debug/pprof` -- replace localhost with the host that you ran the `kubectl port-forward` command on.  The standalone tool can be invoke like this:

```
go tool pprof http://localhost:6060/debug/pprof
```

