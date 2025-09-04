# AWS Deployment Guide

This guide provides step-by-step instructions for deploying sup3rS3cretMes5age on AWS using various services. The application consists of two main components:
- The sup3rS3cretMes5age web application
- A HashiCorp Vault server for secure secret storage

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Option 1: ECS with Fargate (Recommended)](#option-1-ecs-with-fargate-recommended)
3. [Option 2: EKS (Kubernetes)](#option-2-eks-kubernetes)
4. [Option 3: EC2 with Docker](#option-3-ec2-with-docker)
5. [Security Considerations](#security-considerations)
6. [Cost Optimization](#cost-optimization)
7. [Troubleshooting](#troubleshooting)

## Prerequisites

Before starting, ensure you have:

1. **AWS CLI** installed and configured with appropriate permissions
2. **Docker** installed for building and testing images locally
3. **Domain name** (recommended for HTTPS with Let's Encrypt)
4. **AWS Account** with the following IAM permissions:
   - ECS full access
   - ECR full access
   - Application Load Balancer management
   - VPC management
   - IAM role creation
   - Route 53 (if using custom domain)

## Option 1: ECS with Fargate (Recommended)

This option uses AWS ECS with Fargate for a serverless container deployment, which is cost-effective and easy to manage.

### Step 1: Set up ECR Repository

First, create a private ECR repository to store your Docker images:

```bash
# Set your AWS region
export AWS_REGION=us-east-1
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# Create ECR repositories
aws ecr create-repository \
    --repository-name sup3rs3cretmes5age \
    --region $AWS_REGION

aws ecr create-repository \
    --repository-name vault \
    --region $AWS_REGION

# Get login token for ECR
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com
```

### Step 2: Build and Push Docker Images

Build and push the application image to ECR:

```bash
# Clone and build the application
git clone https://github.com/algolia/sup3rS3cretMes5age.git
cd sup3rS3cretMes5age

# Build the application image (with network=host to handle certificate issues)
docker build --network=host -f deploy/Dockerfile -t sup3rs3cretmes5age .

# Tag for ECR
docker tag sup3rs3cretmes5age:latest $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/sup3rs3cretmes5age:latest

# Push to ECR
docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/sup3rs3cretmes5age:latest

# Pull and push Vault image to your ECR
docker pull hashicorp/vault:latest
docker tag hashicorp/vault:latest $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/vault:latest
docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/vault:latest
```

### Step 3: Create VPC and Security Groups

Create a VPC with public and private subnets:

```bash
# Create VPC
export VPC_ID=$(aws ec2 create-vpc \
    --cidr-block 10.0.0.0/16 \
    --query 'Vpc.VpcId' \
    --output text)

aws ec2 create-tags \
    --resources $VPC_ID \
    --tags Key=Name,Value=sup3rs3cretmes5age-vpc

# Create Internet Gateway
export IGW_ID=$(aws ec2 create-internet-gateway \
    --query 'InternetGateway.InternetGatewayId' \
    --output text)

aws ec2 attach-internet-gateway \
    --vpc-id $VPC_ID \
    --internet-gateway-id $IGW_ID

# Create public subnets in two AZs
export PUBLIC_SUBNET_1=$(aws ec2 create-subnet \
    --vpc-id $VPC_ID \
    --cidr-block 10.0.1.0/24 \
    --availability-zone ${AWS_REGION}a \
    --query 'Subnet.SubnetId' \
    --output text)

export PUBLIC_SUBNET_2=$(aws ec2 create-subnet \
    --vpc-id $VPC_ID \
    --cidr-block 10.0.2.0/24 \
    --availability-zone ${AWS_REGION}b \
    --query 'Subnet.SubnetId' \
    --output text)

# Create private subnets
export PRIVATE_SUBNET_1=$(aws ec2 create-subnet \
    --vpc-id $VPC_ID \
    --cidr-block 10.0.3.0/24 \
    --availability-zone ${AWS_REGION}a \
    --query 'Subnet.SubnetId' \
    --output text)

export PRIVATE_SUBNET_2=$(aws ec2 create-subnet \
    --vpc-id $VPC_ID \
    --cidr-block 10.0.4.0/24 \
    --availability-zone ${AWS_REGION}b \
    --query 'Subnet.SubnetId' \
    --output text)

# Create route table for public subnets
export PUBLIC_RT=$(aws ec2 create-route-table \
    --vpc-id $VPC_ID \
    --query 'RouteTable.RouteTableId' \
    --output text)

aws ec2 create-route \
    --route-table-id $PUBLIC_RT \
    --destination-cidr-block 0.0.0.0/0 \
    --gateway-id $IGW_ID

# Associate public subnets with route table
aws ec2 associate-route-table --subnet-id $PUBLIC_SUBNET_1 --route-table-id $PUBLIC_RT
aws ec2 associate-route-table --subnet-id $PUBLIC_SUBNET_2 --route-table-id $PUBLIC_RT

# Enable auto-assign public IPs for public subnets
aws ec2 modify-subnet-attribute --subnet-id $PUBLIC_SUBNET_1 --map-public-ip-on-launch
aws ec2 modify-subnet-attribute --subnet-id $PUBLIC_SUBNET_2 --map-public-ip-on-launch
```

Create security groups:

```bash
# Security group for Application Load Balancer
export ALB_SG=$(aws ec2 create-security-group \
    --group-name sup3rs3cretmes5age-alb-sg \
    --description "Security group for sup3rs3cretmes5age ALB" \
    --vpc-id $VPC_ID \
    --query 'GroupId' \
    --output text)

# Allow HTTP and HTTPS traffic to ALB
aws ec2 authorize-security-group-ingress \
    --group-id $ALB_SG \
    --protocol tcp \
    --port 80 \
    --cidr 0.0.0.0/0

aws ec2 authorize-security-group-ingress \
    --group-id $ALB_SG \
    --protocol tcp \
    --port 443 \
    --cidr 0.0.0.0/0

# Security group for ECS tasks
export ECS_SG=$(aws ec2 create-security-group \
    --group-name sup3rs3cretmes5age-ecs-sg \
    --description "Security group for sup3rs3cretmes5age ECS tasks" \
    --vpc-id $VPC_ID \
    --query 'GroupId' \
    --output text)

# Allow traffic from ALB to ECS tasks
aws ec2 authorize-security-group-ingress \
    --group-id $ECS_SG \
    --protocol tcp \
    --port 80 \
    --source-group $ALB_SG

# Allow Vault communication between tasks
aws ec2 authorize-security-group-ingress \
    --group-id $ECS_SG \
    --protocol tcp \
    --port 8200 \
    --source-group $ECS_SG
```

### Step 4: Create Application Load Balancer

```bash
# Create Application Load Balancer
export ALB_ARN=$(aws elbv2 create-load-balancer \
    --name sup3rs3cretmes5age-alb \
    --subnets $PUBLIC_SUBNET_1 $PUBLIC_SUBNET_2 \
    --security-groups $ALB_SG \
    --query 'LoadBalancers[0].LoadBalancerArn' \
    --output text)

# Get ALB DNS name
export ALB_DNS=$(aws elbv2 describe-load-balancers \
    --load-balancer-arns $ALB_ARN \
    --query 'LoadBalancers[0].DNSName' \
    --output text)

echo "ALB DNS Name: $ALB_DNS"

# Create target group
export TARGET_GROUP_ARN=$(aws elbv2 create-target-group \
    --name sup3rs3cretmes5age-tg \
    --protocol HTTP \
    --port 80 \
    --vpc-id $VPC_ID \
    --target-type ip \
    --health-check-path / \
    --health-check-interval-seconds 30 \
    --health-check-timeout-seconds 5 \
    --healthy-threshold-count 2 \
    --unhealthy-threshold-count 3 \
    --query 'TargetGroups[0].TargetGroupArn' \
    --output text)

# Create listener
aws elbv2 create-listener \
    --load-balancer-arn $ALB_ARN \
    --protocol HTTP \
    --port 80 \
    --default-actions Type=forward,TargetGroupArn=$TARGET_GROUP_ARN
```

### Step 5: Set up ECS Cluster and Task Definitions

Create ECS cluster:

```bash
# Create ECS cluster
aws ecs create-cluster --cluster-name sup3rs3cretmes5age-cluster
```

Create IAM roles for ECS:

```bash
# Create ECS task execution role
cat > ecs-task-execution-role-trust-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

aws iam create-role \
    --role-name ecsTaskExecutionRole \
    --assume-role-policy-document file://ecs-task-execution-role-trust-policy.json

aws iam attach-role-policy \
    --role-name ecsTaskExecutionRole \
    --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy

# Get the role ARN
export TASK_EXECUTION_ROLE_ARN=$(aws iam get-role \
    --role-name ecsTaskExecutionRole \
    --query 'Role.Arn' \
    --output text)
```

Create task definition:

```bash
cat > task-definition.json << EOF
{
  "family": "sup3rs3cretmes5age",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "512",
  "memory": "1024",
  "executionRoleArn": "$TASK_EXECUTION_ROLE_ARN",
  "containerDefinitions": [
    {
      "name": "vault",
      "image": "$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/vault:latest",
      "essential": true,
      "environment": [
        {
          "name": "VAULT_DEV_ROOT_TOKEN_ID",
          "value": "supersecret"
        }
      ],
      "portMappings": [
        {
          "containerPort": 8200,
          "protocol": "tcp"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/sup3rs3cretmes5age",
          "awslogs-region": "$AWS_REGION",
          "awslogs-stream-prefix": "vault"
        }
      },
      "linuxParameters": {
        "capabilities": {
          "add": ["IPC_LOCK"]
        }
      }
    },
    {
      "name": "supersecret",
      "image": "$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/sup3rs3cretmes5age:latest",
      "essential": true,
      "dependsOn": [
        {
          "containerName": "vault",
          "condition": "START"
        }
      ],
      "environment": [
        {
          "name": "VAULT_ADDR",
          "value": "http://localhost:8200"
        },
        {
          "name": "VAULT_TOKEN",
          "value": "supersecret"
        },
        {
          "name": "SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS",
          "value": ":80"
        }
      ],
      "portMappings": [
        {
          "containerPort": 80,
          "protocol": "tcp"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/sup3rs3cretmes5age",
          "awslogs-region": "$AWS_REGION",
          "awslogs-stream-prefix": "supersecret"
        }
      }
    }
  ]
}
EOF

# Create CloudWatch log group
aws logs create-log-group --log-group-name /ecs/sup3rs3cretmes5age

# Register task definition
aws ecs register-task-definition --cli-input-json file://task-definition.json
```

### Step 6: Create ECS Service

```bash
# Create ECS service
cat > service-definition.json << EOF
{
  "serviceName": "sup3rs3cretmes5age-service",
  "cluster": "sup3rs3cretmes5age-cluster",
  "taskDefinition": "sup3rs3cretmes5age",
  "desiredCount": 1,
  "launchType": "FARGATE",
  "networkConfiguration": {
    "awsvpcConfiguration": {
      "subnets": ["$PRIVATE_SUBNET_1", "$PRIVATE_SUBNET_2"],
      "securityGroups": ["$ECS_SG"],
      "assignPublicIp": "DISABLED"
    }
  },
  "loadBalancers": [
    {
      "targetGroupArn": "$TARGET_GROUP_ARN",
      "containerName": "supersecret",
      "containerPort": 80
    }
  ]
}
EOF

aws ecs create-service --cli-input-json file://service-definition.json
```

### Step 7: Configure Domain and HTTPS (Optional but Recommended)

If you have a domain name, you can configure HTTPS:

```bash
# Request SSL certificate (replace with your domain)
export DOMAIN_NAME="secrets.yourdomain.com"

export CERT_ARN=$(aws acm request-certificate \
    --domain-name $DOMAIN_NAME \
    --validation-method DNS \
    --query 'CertificateArn' \
    --output text)

echo "Certificate ARN: $CERT_ARN"
echo "Complete DNS validation in ACM console, then continue..."

# After DNS validation is complete, create HTTPS listener
aws elbv2 create-listener \
    --load-balancer-arn $ALB_ARN \
    --protocol HTTPS \
    --port 443 \
    --certificates CertificateArn=$CERT_ARN \
    --default-actions Type=forward,TargetGroupArn=$TARGET_GROUP_ARN

# Create Route 53 record (if using Route 53)
# You'll need to create this manually or use your DNS provider
```

## Option 2: EKS (Kubernetes)

For teams already using Kubernetes, you can deploy using the provided Helm chart on Amazon EKS.

### Prerequisites for EKS Deployment

```bash
# Install required tools
# - kubectl
# - helm
# - eksctl (recommended for cluster creation)

# Create EKS cluster
eksctl create cluster \
    --name sup3rs3cretmes5age \
    --region $AWS_REGION \
    --nodegroup-name standard-workers \
    --node-type t3.medium \
    --nodes 2 \
    --nodes-min 1 \
    --nodes-max 4 \
    --managed
```

### Deploy with Helm

```bash
# Add Vault Helm repository
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update

# Install Vault
helm install vault hashicorp/vault \
    --set "server.dev.enabled=true" \
    --set "server.dev.devRootToken=supersecret"

# Deploy sup3rS3cretMes5age using the provided Helm chart
cd sup3rS3cretMes5age/deploy/charts/supersecretmessage

# Update values.yaml for your configuration
helm install sup3rs3cretmes5age . \
    --set image.repository=$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/sup3rs3cretmes5age \
    --set image.tag=latest \
    --set vault.address=http://vault:8200 \
    --set vault.token=supersecret

# Create ingress for external access
kubectl apply -f - << EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sup3rs3cretmes5age-ingress
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
spec:
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: sup3rs3cretmes5age
            port:
              number: 80
EOF
```

## Option 3: EC2 with Docker

For a simpler setup, you can deploy on EC2 instances using Docker Compose.

### Step 1: Launch EC2 Instance

```bash
# Create key pair
aws ec2 create-key-pair \
    --key-name sup3rs3cretmes5age-key \
    --query 'KeyMaterial' \
    --output text > sup3rs3cretmes5age-key.pem

chmod 400 sup3rs3cretmes5age-key.pem

# Launch EC2 instance
export INSTANCE_ID=$(aws ec2 run-instances \
    --image-id ami-0c55b159cbfafe1d0 \
    --count 1 \
    --instance-type t3.small \
    --key-name sup3rs3cretmes5age-key \
    --security-groups default \
    --query 'Instances[0].InstanceId' \
    --output text)

# Get public IP
export INSTANCE_IP=$(aws ec2 describe-instances \
    --instance-ids $INSTANCE_ID \
    --query 'Reservations[0].Instances[0].PublicIpAddress' \
    --output text)
```

### Step 2: Configure EC2 Instance

```bash
# SSH to instance and set up Docker
ssh -i sup3rs3cretmes5age-key.pem ec2-user@$INSTANCE_IP

# On the EC2 instance:
sudo yum update -y
sudo yum install -y docker
sudo service docker start
sudo usermod -a -G docker ec2-user

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Clone repository
git clone https://github.com/algolia/sup3rS3cretMes5age.git
cd sup3rS3cretMes5age

# Start services
docker-compose -f deploy/docker-compose.yml up -d
```

## Security Considerations

### 1. Use AWS Secrets Manager for Vault Token

Instead of hardcoding the Vault token, use AWS Secrets Manager:

```bash
# Create secret
aws secretsmanager create-secret \
    --name sup3rs3cretmes5age/vault-token \
    --description "Vault root token for sup3rs3cretmes5age" \
    --secret-string "your-secure-vault-token"

# Update task definition to use secrets
# Add to containerDefinitions[].secrets:
{
  "name": "VAULT_TOKEN",
  "valueFrom": "arn:aws:secretsmanager:region:account:secret:sup3rs3cretmes5age/vault-token"
}
```

### 2. Enable VPC Flow Logs

```bash
aws ec2 create-flow-logs \
    --resource-type VPC \
    --resource-ids $VPC_ID \
    --traffic-type ALL \
    --log-destination-type cloud-watch-logs \
    --log-group-name VPCFlowLogs
```

### 3. Use HTTPS Only

Always configure HTTPS and redirect HTTP traffic:

```bash
# Modify task definition to enable HTTPS redirect
{
  "name": "SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED",
  "value": "true"
}
```

### 4. Implement Network ACLs

Create restrictive network ACLs for additional security:

```bash
# Create network ACL
export NACL_ID=$(aws ec2 create-network-acl \
    --vpc-id $VPC_ID \
    --query 'NetworkAcl.NetworkAclId' \
    --output text)

# Add rules as needed for your security requirements
```

## Cost Optimization

### 1. Use Fargate Spot for Development

For non-production environments, consider using Fargate Spot:

```bash
# Update service to use Fargate Spot
aws ecs put-cluster-capacity-providers \
    --cluster sup3rs3cretmes5age-cluster \
    --capacity-providers FARGATE FARGATE_SPOT
```

### 2. Auto Scaling

Configure auto scaling for production workloads:

```bash
# Register scalable target
aws application-autoscaling register-scalable-target \
    --service-namespace ecs \
    --scalable-dimension ecs:service:DesiredCount \
    --resource-id service/sup3rs3cretmes5age-cluster/sup3rs3cretmes5age-service \
    --min-capacity 1 \
    --max-capacity 10

# Create scaling policy
aws application-autoscaling put-scaling-policy \
    --policy-name sup3rs3cretmes5age-scaling-policy \
    --service-namespace ecs \
    --scalable-dimension ecs:service:DesiredCount \
    --resource-id service/sup3rs3cretmes5age-cluster/sup3rs3cretmes5age-service \
    --policy-type TargetTrackingScaling \
    --target-tracking-scaling-policy-configuration file://scaling-policy.json
```

### 3. Use Reserved Instances for EC2

For long-running deployments, consider Reserved Instances to reduce costs.

## Troubleshooting

### Common Issues

1. **Service won't start**: Check CloudWatch logs for container errors
2. **Can't access application**: Verify security group rules and target group health
3. **SSL certificate issues**: Ensure DNS validation is complete
4. **Vault connection errors**: Check network connectivity between containers

### Debugging Commands

```bash
# Check ECS service status
aws ecs describe-services \
    --cluster sup3rs3cretmes5age-cluster \
    --services sup3rs3cretmes5age-service

# View logs
aws logs tail /ecs/sup3rs3cretmes5age --follow

# Check target group health
aws elbv2 describe-target-health \
    --target-group-arn $TARGET_GROUP_ARN

# Test internal connectivity
aws ecs execute-command \
    --cluster sup3rs3cretmes5age-cluster \
    --task <task-id> \
    --container supersecret \
    --interactive \
    --command "/bin/sh"
```

### Support Resources

- [AWS ECS Documentation](https://docs.aws.amazon.com/ecs/)
- [HashiCorp Vault Documentation](https://www.vaultproject.io/docs)
- [Application Load Balancer Documentation](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/)

---

This guide provides multiple deployment options on AWS. Choose the option that best fits your team's expertise and requirements. For production deployments, we recommend Option 1 (ECS with Fargate) for its balance of simplicity, scalability, and cost-effectiveness.