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
    - [ ] simple API example (data analysis)
    - [ ] classic Mortar API example

## Docker Compose to K8s

- install [`kompose`](https://github.com/kubernetes/kompose)
- run `kompose convert -f docker-compose.yml -o k8s`


