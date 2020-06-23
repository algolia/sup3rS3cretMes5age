# sup3rS3cretMes5age! [![Build Status](https://travis-ci.org/algolia/sup3rS3cretMes5age.svg)](https://travis-ci.org/algolia/sup3rS3cretMes5age)

A simple, secure self-destructing message service, using HashiCorp Vault product as a backend.

![self-destruct](https://media.giphy.com/media/LBlyAAFJ71eMw/giphy.gif)

Read more about the reasoning behind this project in the [relevant](https://blog.algolia.com/secure-tool-for-one-time-self-destructing-messages/) blog post.

Now using Let's Encrypt for simple and free SSL certs!

#### Prerequisites

* [Go](https://golang.org/doc/install) (for development)
* [Docker](https://docs.docker.com/engine/installation/)
* [Docker-Compose](https://docs.docker.com/compose/install/)
* Make

#### Running Locally

Running locally will use a self-signed SSL certificate for whatever your local dev domain is. The default is `localhost`, to change it just pass an argument to `make`. For example, if you set `127.0.0.1 secret.test` in your `/etc/hosts` you would run locally as:

```shell
$ export VIRTUAL_HOST=secret.test
$ make run-local 
```

Try it! (you can ignore the safety warning since it's a self-signed cert)

```shell
https://secret.test
```

#### Running with Let's Encrypt

Using [lets-encrypt-nginx-proxy-companion](https://github.com/JrCs/docker-letsencrypt-nginx-proxy-companion) you can now get a free (and valid) SSL cert when running this project on a live server. Thanks to [evertramos](https://github.com/evertramos/)'s [docker-compose-letsencrypt-nginx-proxy-companion](https://github.com/evertramos/docker-compose-letsencrypt-nginx-proxy-companion) for a great working example.

1. Clone this repo
1. Ensure you have `docker` and `docker-compose` installed on server
1. run `VIRTUAL_HOST=<YOUR_DOMAIN_HERE>
1. run `make run` 
1. Let's Encrypt may take a few minutes to validate your domain
1. open `https://your-domain`


### Security notice!

You should always run this behind SSL/TLS; otherwise, a message will be sent unencrypted!

### Screenshot

<img width="610" alt="secretmsg" src="https://user-images.githubusercontent.com/357094/29357449-e9268adc-8277-11e7-8fef-b1eabfe62444.png">

### Contributing

Pull requests are very welcome!
