# sup3rS3cretMes5age

A simple, secure self-destructing message service, using HashiCorp Vault product as a backend.

![self-destruct](https://media.giphy.com/media/LBlyAAFJ71eMw/giphy.gif)

Read more about the reasoning behind this project in the [relevant blog post](https://blog.algolia.com/secure-tool-for-one-time-self-destructing-messages/).

Now using [Let's Encrypt](https://letsencrypt.org/) for simple and free SSL certs!

## Deployment

### Testing it locally

You can just run `docker-compose up -f deploy/docker-compose.yml --build` or run `make build`: it will build the Docker image and then run it alongside a standalone Vault server.

By default, the `deploy/docker-compose.yml` is configured to run the webapp on port 8082 in cleartext HTTP (so you can access it on [http://localhost:8082](http://localhost:8082)).

Optionally, you can modify the `deploy/docker-compose.yml` and tweak the options (enable HTTPS, disable HTTP or enable redirection to HTTPS, etc.). See [Configuration options](#configuration-options).

### Production Deployment

We recommend deploying the project via **Docker** and a **container orchestration tool**:

* Build the Docker image using the provided `Dockerfile` or run `make image`
* Host it in a Docker registry ([Docker Hub](https://hub.docker.com/), [AWS ECR](https://aws.amazon.com/ecr/), etc.)
* Deploy the image (alongside with a standalone Vault server) using a container orchestration tool ([Kubernetes](https://kubernetes.io/), [Docker Swarm](https://docs.docker.com/engine/swarm/), [AWS ECS](https://aws.amazon.com/ecs/), etc.)

You can read the [configuration examples](#configuration-examples) below.

### Security notice

Whatever deployment method you choose, **you should always run this behind SSL/TLS**, otherwise secrets will be sent _unencrypted_!

Depending on your infrastructure/deployment, you can have **TLS termination** either _inside the container_ (see [Configuration examples - TLS](#tls)), or _before_ e.g. at a load balancer/reverse proxy in front of the service.
It is interesting to have TLS termination before the container so you don't have to manage the certificate/key there, but **make sure the network** between your TLS termination point and your container **is secure**.

## Helm

For full documentation for this chart, please see the [README](https://github.com/algolia/sup3rS3cretMes5age/blob/master/deployments/charts/README.md)

## Command Line Usage

For convenient command line integration and automation, see our comprehensive [CLI Guide](CLI.md) which includes shell functions for Bash, Zsh, Fish, and WSL.

Quick example:
```bash
# Add to your ~/.bashrc or ~/.zshrc
o() { cat "$@" | curl -sF 'msg=<-' https://your-domain.com/secret | jq -r .token | awk '{print "https://your-domain.com/getmsg?token="$1}'; }

# Usage
echo "secret message" | o
o secret-file.txt
```

## Configuration options

* `VAULT_ADDR`: address of the Vault server used for storing the temporary secrets.
* `VAULT_TOKEN`: Vault token used to authenticate to the Vault server.
* `SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS`: HTTP binding address (e.g. `:80`).
* `SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS`: HTTPS binding address (e.g. `:443`).
* `SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED`: whether to enable HTTPS redirection or not (e.g. `true`).
* `SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN`: domain to use for "Auto" TLS, i.e. automatic generation of certificate with Let's Encrypt. See [Configuration examples - TLS - Auto TLS](#auto-tls).
* `SUPERSECRETMESSAGE_TLS_CERT_FILEPATH`: certificate filepath to use for "manual" TLS.
* `SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH`: certificate key filepath to use for "manual" TLS.
* `SUPERSECRETMESSAGE_VAULT_PREFIX`: vault prefix for secrets (default `cubbyhole/`)

## Configuration examples

Here is an example of a functionnal docker-compose.yml file
```yaml
version: '3.2'

services:
  vault:
    image: vault:latest
    container_name: vault
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: root
    cap_add:
      - IPC_LOCK
    expose:
      - 8200

  supersecret:
    build: ./
    image: algolia/supersecretmessage:latest
    container_name: supersecret
    environment:
      VAULT_ADDR: http://vault:8200
      VAULT_TOKEN: root
      SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS: ":80"
      SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS: ":443"
      SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED: "true"
      SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN: secrets.example.com
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - vault
```

### Configuration types

#### Plain HTTP

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=:80
```

#### TLS

##### Auto TLS

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS=:443
SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN=secrets.example.com
```

##### Auto TLS with HTTP > HTTPS redirection

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTP_BINDING_ADDRESS=:80
SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS=:443
SUPERSECRETMESSAGE_HTTPS_REDIRECT_ENABLED=true
SUPERSECRETMESSAGE_TLS_AUTO_DOMAIN=secrets.example.com
```

##### Manual TLS

```bash
VAULT_ADDR=http://vault:8200
VAULT_TOKEN=root

SUPERSECRETMESSAGE_HTTPS_BINDING_ADDRESS=:443
SUPERSECRETMESSAGE_TLS_CERT_FILEPATH=/mnt/ssl/cert_secrets.example.com.pem
SUPERSECRETMESSAGE_TLS_CERT_KEY_FILEPATH=/mnt/ssl/key_secrets.example.com.pem
```

## Screenshot

![supersecretmsg](https://user-images.githubusercontent.com/357094/29357449-e9268adc-8277-11e7-8fef-b1eabfe62444.png)

## Contributing

Pull requests are very welcome!
Please consider that they will be reviewed by our team at Algolia.


## Thanks

This project is heavaily depandent on the amazing work of the [Echo Go Web Framework](https://github.com/labstack/echo) and Hashicorp Vault. 

[![This project is certified Awesome F/OSS](https://awsmfoss.com/content/images/2024/02/awsm-foss-badge.600x128.rounded.png)](https://awsmfoss.com/sup3rs3cretmes5age)
