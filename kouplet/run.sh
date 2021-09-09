#!/bin/bash

git pull

./build.sh

~/operator-sdk run local --watch-namespace=jgw 2>&1 | tee /tmp/op.log | java -jar ../operator-log-parser/target/operator-log-parser-0.0.1-SNAPSHOT.jar

