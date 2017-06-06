# secretMsg

A simple secure self destructing service message server, using hashicorp vault as a backend


### Run localy 



* run hashicorp vault server 

    ` docker run -d --cap-add=IPC_LOCK -ti -p 8200:8200   --name vault vault `

* set vault environment variable 

   ```shell 
    export VAULT_ADDR=http://localhost:8200 
    export VAULT_TOKEN=<token_from_vault_server_log> 
   ```

* run the secretMsg service
  ```shell
  go get ; go run *.go 
	```

* use it!

	`http://localhost:1234/msg`
