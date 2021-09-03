#!/bin/bash
docker build -t kouplet-builder-util .

docker tag kouplet-builder-util jgwest/k-builder-util

docker push jgwest/k-builder-util

