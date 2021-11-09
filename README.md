# Mortar

Beta documentation here: https://beta.mortardata.org/intro.html

## Tips
- rebuild/restart a single service with:
    ```
    docker-compose up --detach --build mortar-server
    ```
- which interface is a docker container on:
    ```
    docker exec -ti mortar_reasoner_1 cat /sys/class/net/eth0/iflink
    ```
    - then use the returned number to find the correct interface in `ip a`

## TODOs:
- [ ] add API call to get all metadata for an entity
- [ ] preload some common views
- [ ] Jupyter notebooks preloaded, 1 for each major piece of functionality:
    - [X] loading turtle files
    - [X] loading data files
    - [X] simple API example (data analysis)
    - [ ] classic Mortar API example

## Docker Compose to K8s

- install [`kompose`](https://github.com/kubernetes/kompose)
- run `kompose convert -f docker-compose.yml -o k8s`

## Running Examples
In order to query and write to the Postgres database, you must create and authorize a new API key.
`$ docker exec -ti mortar_pg_1 psql mortar -U <username>`
`mortar=# SELECT new_apikey();`
`mortar=# SELECT authorize_write('<your_new_apikey>', '<sitename>');`
