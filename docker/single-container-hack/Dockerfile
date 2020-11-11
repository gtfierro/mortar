FROM ubuntu:19.10

RUN echo "debconf debconf/frontend select Noninteractive" | debconf-set-selections
RUN apt-get update && \
    apt-get install -y git

WORKDIR /app
COPY --from=library/docker:latest /usr/local/bin/docker /usr/bin/docker
COPY --from=docker/compose:1.23.2 /usr/local/bin/docker-compose /usr/bin/docker-compose

RUN git clone https://github.com/gtfierro/mortar 
WORKDIR /app/mortar
CMD ["/usr/bin/docker-compose", "up",  "--build", "--force-recreate"]
