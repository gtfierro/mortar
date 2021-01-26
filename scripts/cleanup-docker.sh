#!/bin/bash

echo "THIS SCRIPT WILL NUKE ALL 'mortar2' DOCKER CONTAINERS AND IMAGES"
read -p "Do you want to continue? [yn]" -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo "removing all mortar2 containers"
    docker ps -a | grep mortar2 | awk '{print $1}' | xargs docker rm  2>/dev/null
    if [[ $? -ne 0 ]] ; then
        echo "No containers found"
    fi
    echo "removing all mortar2 images"
    docker images | grep mortar | awk '{print $1}' | xargs docker rmi 2>/dev/null
    if [[ $? -ne 0 ]] ; then
        echo "No images found"
    fi
fi
