## Steps to reproduce

1. Run `export AWS_PROFILE=<profile>`
2. Run `copilot app init`
3. Enable Container Insights in the cluster [manifest.yaml](./copilot/environments/production/manifest.yml#L20-L21)
```yaml
observability:
  container_insights: true
```
4. Run `copilot env init` (with the same name that already exists) _--> create roles_
5. Run `copilot env deploy --name production` _--> create the environment_
6. Export cluster variables: <br/>
```bash
export CLUSTER_NAME=$(aws ecs list-clusters --query "clusterArns[0]" --output text | cut -d'/' -f2)
export CLUSTER_REGION=$(aws ecs describe-clusters --cluster $CLUSTER_NAME --query "clusters[0].clusterArn" --output text | cut -d':' -f4)
export CLUSTER_ARN=$(aws ecs list-clusters | jq -r '.clusterArns[]')
export AWS_ACCOUNT=$(aws ecs describe-clusters --cluster $CLUSTER_NAME --query "clusters[0].clusterArn" --output text | cut -d':' -f5)
```

7. Create a Capacity Provider _--> to define the infra (SPOT) where the tasks will run_
```bash
aws ecs put-cluster-capacity-providers \
  --cluster $CLUSTER_NAME \
  --capacity-providers FARGATE_SPOT \
  --default-capacity-provider-strategy \
  capacityProvider=FARGATE_SPOT,weight=4
```

8. Run `copilot deploy` _--> deploy the application_
9. Install CloudWatch agent
```bash
aws cloudformation create-stack \
  --stack-name CWAgentECS-$CLUSTER_NAME-$CLUSTER_REGION \
  --template-body "$(curl -Ls https://raw.githubusercontent.com/aws-samples/amazon-cloudwatch-container-insights/latest/ecs-task-definition-templates/deployment-mode/daemon-service/cwagent-ecs-instance-metric/cloudformation-quickstart/cwagent-ecs-instance-metric-cfn.json)" \
  --parameters ParameterKey=ClusterName,ParameterValue=$CLUSTER_NAME ParameterKey=CreateIAMRoles,ParameterValue=True \
  --capabilities CAPABILITY_NAMED_IAM --region $CLUSTER_REGION
```
## Configure Autoscaling

1. Export service variables: <br/>
```bash
export SVC_NAME=$(aws ecs list-services --cluster $CLUSTER_NAME --query "serviceArns[0]" --output text | cut -d'/' -f3)
export LOG_GROUP='/copilot/ecs-demo-production-api'
export POLICY_NAME=HTTP-Request-Rate-Policy
```

2. Register target
```bash
aws application-autoscaling register-scalable-target \
  --service-namespace ecs \
  --scalable-dimension ecs:service:DesiredCount \
  --resource-id service/$CLUSTER_NAME/$SVC_NAME \
  --min-capacity 2 \
  --max-capacity 10 \
  --region $CLUSTER_REGION
```

3. Create the log filter
```bash
aws logs put-metric-filter \
  --log-group-name $LOG_GROUP \
  --filter-name HttpRequestFilter \
  --filter-pattern '%REQUEST RECEIVED: Handling HTTP request%' \
  --metric-transformations metricName=http_request,metricNamespace=ecs,metricValue=1
```

4. Create [`scaling-policy.json`](./scaling-policy.json) with the following content
```json
1. | {
2. |   "TargetValue": 20.0,
3. |   "CustomizedMetricSpecification": {
4. |     "Namespace": "ecs",
5. |     "MetricName": "RequestByTaskGreaterThan20",
6. |     "Statistic": "Sum",
7. |     "Unit": "Count"
8. |   },
9. |   "ScaleOutCooldown": 300,
10.|   "ScaleInCooldown": 60
11.| }

```
> The `MetricName` refers to the custom metric we've created in Cloudwatch console that contains <br/>
> `(customMetricByLog / RunningTaskCount)`

3. Create scaling policy
```bash
aws application-autoscaling put-scaling-policy \
  --service-namespace ecs \
  --resource-id service/$CLUSTER_NAME/$SVC_NAME \
  --scalable-dimension ecs:service:DesiredCount \
  --policy-name $POLICY_NAME \
  --policy-type TargetTrackingScaling \
  --target-tracking-scaling-policy-configuration file://scaling-policy.json
```

## 🧪 Load test
1. Install Siege
```bash
🍏 $ brew install siege
$ - -
🐧 $ apt-get install siege
```
2. Send the load for 3 minutes
```bash
siege -b -t180S $LOAD_BALANCER
```

## 🧹 Clean Up
```bash
copilot app delete
```

## ⌨️ Cheat sheet
```bash
# List tasks
aws ecs list-tasks --cluster $CLUSTER_NAME --service-name $SVC_NAME --query "taskArns[]" --output text | xargs aws ecs describe-tasks --cluster $CLUSTER_NAME --tasks \| jq -r '.tasks[].containers[0].name'`
# List autoscaling policies
aws application-autoscaling describe-scaling-policies --service-namespace ecs --resource-id service/$CLUSTER_NAME/$SVC_NAME --scalable-dimension ecs:service:DesiredCount
```

----
## Dúvidas
- como funciona a manutenção das tasks-definitions?
  - caso eu precise usar alguma configuração como padrão pra todas* as apps, como isso pode ser feito?
  - como dar um nome amigável para os componentes (task-definitions, services, etc)?
- usa alguma pipeline?
- como funciona o fluxo de branch?