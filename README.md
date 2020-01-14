# Running locally
`skaffold dev`

Send a test message:

`aws sqs send-message --endpoint-url http://localhost:30576 --queue-url http://localhost:30576/queue/rode-ecr-event-ingester --message-body file://test/sample_scan_event.json`

# Running on remote cluster
`skaffold dev -d xxxxx.dkr.ecr.us-east-1.amazonaws.com -p production`

# IAM Policy
The ingester pod requires the following IAM policy:

```
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
