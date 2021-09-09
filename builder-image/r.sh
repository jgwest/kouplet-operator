#!/bin/bash

docker run --rm -it --privileged -v build-volume:/var/lib/containers -v ~/.ssh:/ssh-credentials kouplet-builder-util # bash

