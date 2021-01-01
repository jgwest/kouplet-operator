
CONTAINER_REGISTRY_NAMESPACE?=jgwest
CONTAINER_IMAGE?=k-builder-util


.PHONY: build-image
build-image:
	docker build -t ${CONTAINER_IMAGE} .
	docker tag ${CONTAINER_IMAGE} ${CONTAINER_REGISTRY_NAMESPACE}/${CONTAINER_IMAGE}


.PHONY: build-image-push
build-image-push: build-image
	docker push ${CONTAINER_REGISTRY_NAMESPACE}/${CONTAINER_IMAGE}


