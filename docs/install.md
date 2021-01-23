Installation
============


## Development Installation

1. Install `docker-compose` using the [online instructions](https://docs.docker.com/compose/install/)
2. Clone the `mortar` repo: (`git clone https://github.com/gtfierro/mortar`)
3. Edit any usernames, passwords and ports as needed in `docker-compose.yml` (these each show up three times so make sure they are the same!)
4. Bring the server up with `docker-compose up`:
    - Note that if you change the user/password after the first run, you will need to rebuild the postgres container using `docker-compose build pg`
5. Use `docker ps` to ensure that the services are all running:
    ```
    $ docker ps
    CONTAINER ID        IMAGE                    COMMAND                  CREATED              STATUS              PORTS                    NAMES
    31f5b7387bb5        mortar/reasoner          "./reasoner"             About a minute ago   Up 1 second         0.0.0.0:3030->3030/tcp   mortar2_reasoner_1
    31e25d03cefa        mortar/server2           "./mortar-server"        About a minute ago   Up 1 second         0.0.0.0:5001->5001/tcp   mortar2_mortar-server_1
    ab14de8bfbba        jupyter/scipy-notebook   "tini -g -- start-no…"   9 minutes ago        Up 2 seconds        0.0.0.0:8888->8888/tcp   mortar2_jupyter-notebook_1
    c8fe7f3a04f3        prom/prometheus:latest   "/bin/prometheus --c…"   5 days ago           Up 1 second         0.0.0.0:9090->9090/tcp   prometheus
    4670aa9b5d2e        google/cadvisor:latest   "/usr/bin/cadvisor -…"   5 days ago           Up 1 second         0.0.0.0:8080->8080/tcp   cadvisor
    79a782bef10b        mortar/pg                "docker-entrypoint.s…"   5 days ago           Up 2 seconds        0.0.0.0:5434->5432/tcp   mortar2_pg_1
    ```

If you make changes to any of the source code, make sure to use `docker-compose up --build` to ensure that the containers are rebuilt
