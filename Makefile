.PHONY: build-operator-parser
build-operator-parser:
	cd operator-log-parser && mvn package

.PHONY: build-operator
build-operator:
	cd kouplet && make build



.PHONY: build
build: build-operator-parser build-operator 

