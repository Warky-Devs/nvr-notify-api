#!/bin/sh


docker build . --build-arg CACHEBUST=$(date +%s) -t localdev/nvr-api
echo Installing....
docker stop nvr-api
docker rm nvr-api
#docker volume create --name nvr-api

#docker run -d -p 8082:8080 -v /tmp/config.json:/app/config.json --name nvr-api --restart unless-stopped --memory=2G --cpus=1 localdev/nvr-api
docker run -d -p 8082:8080 --name nvr-api --restart unless-stopped --memory=2G --cpus=1 localdev/nvr-api

