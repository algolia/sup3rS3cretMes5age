# sup3rS3cretMes5age! [![Build Status](https://travis-ci.org/algolia/sup3rS3cretMes5age.svg)](https://travis-ci.org/algolia/sup3rS3cretMes5age)

A simple, secure self-destructing message service, using HashiCorp Vault product as a backend.

![self-destruct](https://media.giphy.com/media/LBlyAAFJ71eMw/giphy.gif)

Read more about the reasoning behind this project in the [relevant](https://blog.algolia.com/secure-tool-for-one-time-self-destructing-messages/) blog post.

Now using Let's Encrypt for simple and free SSL certs!

#### Prerequisites

* [Go](https://golang.org/doc/install) (just for development)
* [Docker](https://docs.docker.com/engine/installation/)
* [Docker-Compose](https://docs.docker.com/compose/install/)
* Make

#### Running Locally

Running locally will use a self-signed SSL certificate for `localhost` only. 

```shell
$ make run-local
```

Try it! (you can ignore the safety warning since it's a self-signed cert)

```shell
https://localhost
```

#### Running with Let's Encrypt


1. Clone this repo
2. Ensure you have `docker` and `docker-compose` installed on server
3. run `DOMAIN=secret.example.com make run`
4. Let's Encrypt may take a few minutes to validate your domain
5. open `https://secret.example.com`


### Security notice!

You should always run this behind SSL/TLS; otherwise, a message will be sent unencrypted!

### Screenshot

<img width="610" alt="secretmsg" src="https://user-images.githubusercontent.com/357094/29357449-e9268adc-8277-11e7-8fef-b1eabfe62444.png">

### Contributing

Pull requests are very welcome!
