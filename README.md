# Warning: this project as been deprecated
As of 12/18/2020 the new home for all `rode` development will be located at [github.com/rode](https://github.com/rode) 

# ![](docs/logo.png) Rode 
![tag](https://github.com/liatrio/rode/workflows/tag/badge.svg)
> \rōd\ - a line (as of rope or chain) used to attach an anchor to a boat

Rode provides the collection, attestation and enforcement of policies in your software supply chain.  Watch the [demo](https://youtu.be/CyrbLQYUCbM?t=580) and [slides](https://www.slideshare.net/CaseyLee2/the-last-bottleneck-of-continuous-delivery/) from DeliveryConf for a quick introduction!

![](docs/overview.png)

There are 3 primary components in rode: `collectors`, `attesters` and `enforcers`

## Collectors
Collectors are responsible for receiving events from external systems and converting them into [occurrences](https://github.com/grafeas/grafeas/blob/master/docs/grafeas_concepts.md#occurrences) in [Grafeas](https://github.com/grafeas/grafeas).

![](docs/collectors.png)

The list of supported collectors is growing and currently includes:
* **ECR Events** - image scan events are sent to an SQS queue via CloudWatch event rules.  A collector in rode processes the messages from the queue and converts them into [discovery](https://github.com/grafeas/grafeas/blob/master/docs/grafeas_concepts.md#kind-specific-schemas) and [vulnerability](https://github.com/grafeas/grafeas/blob/master/docs/grafeas_concepts.md#kind-specific-schemas) occurrences in Grafeas.
* **Harbor Events** - image scan events are sent to a Rode endpoint.  A collector in rode processes the messages from the queue and converts them into [discovery](https://github.com/grafeas/grafeas/blob/master/docs/grafeas_concepts.md#kind-specific-schemas) and [vulnerability](https://github.com/grafeas/grafeas/blob/master/docs/grafeas_concepts.md#kind-specific-schemas) occurrences in Grafeas.

Collectors are defined as `Collector` [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).  See below for an example:

```yaml
apiVersion: rode.liatr.io/v1alpha1
kind: Collector
spec:
  name: my_collector
  type: ecr
  queueName: my_ecr_event_queue
```

## Attesters
Attesters monitor collectors for new `occurrences`.  Whenever a new occurrence is created on a [resource](https://github.com/grafeas/grafeas/blob/master/docs/grafeas_concepts.md#resource-urls), then all occurrences are loaded for that resource and passed in to [Open Policy Agent (OPA)](https://www.openpolicyagent.org/) to determine if all necessary occurrences exist for the resource.

If all occurrences exist and comply with the policy, then the attester will use its private PGP key to sign a new attestation for the resource and store the attestation in Grafeas.

![](docs/attesters.png)

Attesters are defined as `Attester` [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).  See below for an example:

```yaml
apiVersion: rode.liatr.io/v1alpha1
kind: Attester
spec:
  name: my_collector
  pgp-secret: my_secret_name
  policy: |
    package my_collector

    violation[{"msg":"analysis failed"}]{
        input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
    }
    violation[{"msg":"analysis not performed"}]{
        analysisStatus := [s | s := input.occurrences[_].discovered.discovered.analysisStatus]
        count(analysisStatus) = 0
    }
    violation[{"msg":"critical vulnerability found"}]{
        severityCount("CRITICAL") > 0
    }
    violation[{"msg":"high vulnerability found"}]{
        severityCount("HIGH") > 10
    }
    severityCount(severity) = cnt {
        cnt := count([v | v := input.occurrences[_].vulnerability.severity; v == severity])
    }
```

The PGP key is automatically generated and stored as a Kubernetes secret if it doesn't already exist.

## Enforcers
Enforcers are defined as [validating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) that ensures the resource defined as an `image` in the `Pod` has been properly attested.

Enforcers are configured to ensure the specified attester referenced in the namespace for the pod had successfully created an attestation. The namespace must include a label for enforcement to be activated:


```yaml
  "rode.liatr.io/enforce": true
```

![](docs/enforcers.png)

# Installation
The easiest way to install rode is via the helm chart:

```shell
helm repo add liatrio https://harbor.toolchain.lead.prod.liatr.io/chartrepo/public
helm upgrade -i rode liatrio/rode
```

## Elastic Container Registry

Setup collectors, attesters and enforcers through a quickstart:

```shell
kubectl apply -f examples/aws-quickstart.yaml
```

The ECR event collector requires the following IAM policy.  Either attach the policy to the EC2 instance or use IRSA and pass the role ARN to Helm:

```shell
helm upgrade -i rode liatrio/rode --set rbac.serviceAccountAnnotations."eks\.amazonaws\.com/role-arn"=arn:aws:iam::1234567890:role/RodeServiceAccount
```

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "sqs:CreateQueue"
                "sqs:SetQueueAttributes",
                "sqs:GetQueueUrl",
                "sqs:GetQueueAttributes",
                "sqs:ReceiveMessage",
                "sqs:DeleteMessage",
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "events:PutTargets",
                "events:PutRule"
            ],
            "Resource": "*"
        }
    ]
}
```

## Harbor

If Harbor is being utilized as a container registry, you can specify `harbor` as the collector type.

```yaml
apiVersion: rode.liatr.io/v1alpha1
kind: Collector
spec:
  name: my_collector
  type: ecr
  queueName: my_ecr_event_queue
---
apiVersion: rode.liatr.io/v1alpha1
kind: Collector
metadata: 
  name: harborCollector
  finalizers:
  - collectors.finalizers.rode.liatr.io
spec:
  harbor:
    harborUrl: "https://example.com"
    project: "example-project"
    secret: "default/harbor-harbor-core"
  type: harbor

```

# Development
To run locally you need to have Docker for Desktop running and your k8s context set to `docker-desktop`. Skaffold will automatically use the `local` profile which installs [LocalStack](https://github.com/localstack/localstack) to test the AWS ECR collector by mocking the SQS service.

To run controllers:

```shell
skaffold dev --port-forward
```

Setup collectors, attesters and enforcers:

```shell
kubectl apply -f examples/aws-quickstart.yaml
```

To create an occurrence, use the aws cli to send a test message to LocalStack:

```shell
aws sqs send-message \
    --endpoint-url http://localhost:30576 \
    --queue-url http://localhost:30576/queue/rode-ecr-event-collector  \
    --message-body file://test/sample_scan_event.json 
``` 

## Testing

To run unit tests 

```shell
go test -cover -tags unit ./...
```

To run unit and integration tests Rode and LocalStack must be running. Follow the local development instructions above.
```shell
go test ./...
```

## Testing Multi-Cluster Communication

You can test separating the policy and enforcement features of Rode by running both locally. This will deploy different instances of Rode into two different namespaces (`rode-policy` and `rode-enforcement`) so you can test the communication between the two instances.

**Deploy the policy Rode instance**

```shell
skaffold run -p local,policy
```

This will deploy Rode with Collector and Attester controllers and deploy the messaging service [Jetstream](https://github.com/nats-io/jetstream)

**Deploy the enforcement Rode instance**

```shell
skaffold run -p local,enforcement
```

This will deploy Rode with Enforcer and Cluster Enforcer controllers and the enforcer validating webhook. 
