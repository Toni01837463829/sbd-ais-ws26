#!/bin/sh

# docker build
docker build -t orderservice .

# docker run db
docker rm -f orderdb 2>/dev/null
docker run -d --name orderdb --network host --env-file debug.env \
  -v orderdb-data:/var/lib/postgresql/18/docker postgres:18

# docker run orderservice
docker rm -f orderservice 2>/dev/null
docker run -d --name orderservice --network host --env-file debug.env orderservice
