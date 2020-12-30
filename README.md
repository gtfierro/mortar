# Mortar

Beta documentation here: https://beta.mortardata.org/intro.html

## TODOs:
- [ ] add API call to get all metadata for an entity
- [ ] preload some common views
- [ ] Jupyter notebooks preloaded, 1 for each major piece of functionality:
    - [ ] loading turtle files
    - [ ] loading data files
    - [ ] simple API example (data analysis)
    - [ ] classic Mortar API example

## Docker Compose to K8s

- install [`kompose`](https://github.com/kubernetes/kompose)
- run `kompose convert -f docker-compose.yml -o k8s`


