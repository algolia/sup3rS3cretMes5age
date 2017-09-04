# sup3rS3cretMes5age!

A simple, secure self-destructing message service, using HashiCorp Vault product as a backend.

![self-destruct](https://media.giphy.com/media/LBlyAAFJ71eMw/giphy.gif)

Read more about the reasoning behind this project in the [relevant](https://blog.algolia.com/secure-tool-for-one-time-self-destructing-messages/) blog post.

### Run locally 

#### Prerequisites

* [Go](https://golang.org/doc/install)
* [Docker](https://docs.docker.com/engine/installation/)

#### Installing

* run hashicorp vault server 

    ```shell
    docker run -ti --cap-add=IPC_LOCK -ti -p 8200:8200   --name vault vault
    ```

* set vault environment variable 

    ```shell 
    export VAULT_ADDR=http://localhost:8200
    export VAULT_TOKEN=$(docker logs vault | grep Token | awk '{print $NF}')
    ```

* run the secretMsg service
    ```shell
    git clone https://github.com/algolia/sup3rS3cretMes5age.git
    go get
    go run *.go
    ```

* try it!

    ```shell
    http://localhost:1234/msg
    ```
    
    
### Security notice!

You should always run this behind SSL/TLS; otherwise, a message will be sent unencrypted!


### Screenshot

<img width="610" alt="secretmsg" src="https://user-images.githubusercontent.com/357094/29357449-e9268adc-8277-11e7-8fef-b1eabfe62444.png">

### Contributing

Pull requests are very welcome!
