Installation
============


## Development Installation

1. Install `docker-compose` using the [online instructions](https://docs.docker.com/compose/install/)
2. Clone the `mortar` repo: (`git clone https://github.com/gtfierro/mortar`)
3. Edit any usernames, passwords and ports as needed in `docker-compose.yml` (these each show up twice so make sure they are the same!)
4. Bring the server up with `docker-compose up`
5. Use `docker ps` to ensure that the services are all running:
    ```
    $ docker ps
    CONTAINER ID        IMAGE                    COMMAND                  CREATED             STATUS              PORTS                    NAMES
    cca2b2a1a850        mortar/reasoner          "./reasoner"             5 days ago          Up 6 hours          0.0.0.0:3030->3030/tcp   mortar2_reasoner_1
    954510d4a3c5        mortar/server2           "./mortar-server"        5 days ago          Up 6 hours          0.0.0.0:5001->5001/tcp   mortar2_server_1
    c8fe7f3a04f3        prom/prometheus:latest   "/bin/prometheus --c…"   5 days ago          Up 6 hours          0.0.0.0:9090->9090/tcp   prometheus
    4670aa9b5d2e        google/cadvisor:latest   "/usr/bin/cadvisor -…"   5 days ago          Up 6 hours          0.0.0.0:8080->8080/tcp   cadvisor
    79a782bef10b        mortar/pg                "docker-entrypoint.s…"   5 days ago          Up 6 hours          0.0.0.0:5434->5432/tcp   mortar2_pg_1
    ```

If you make changes to any of the source code, make sure to use `docker-compose up --build` to ensure that the containers are rebuilt
