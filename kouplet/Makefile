NAMESPACE?=kouplet


.PHONY: build
build:
	go build ./...

.PHONY: run-local
run-local:  build
	operator-sdk run local --watch-namespace=${NAMESPACE} 2>&1 | java -jar ../operator-log-parser/target/operator-log-parser-0.0.1-SNAPSHOT.jar

.PHONY: regenerate-manifests
regenerate-manifests: 
	operator-sdk generate k8s
	operator-sdk generate crds


