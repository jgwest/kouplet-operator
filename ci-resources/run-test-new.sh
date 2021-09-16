#!/bin/bash

print_status() {
	echo
	echo "$*"
}

dispose () {
	set +euo pipefail

	cd "$ROOT/../ci-resources"

	if [ -n "$REMOVE_IMAGE_CLI" ]; then

		$REMOVE_IMAGE_CLI "$REGISTRY_HOST_PREFIX""$REGISTRY_NAMESPACE"/kouplet-builder-util-test:"$TEST_UUID"

		$REMOVE_IMAGE_CLI "$CONTAINER_IMAGE"

		$REMOVE_IMAGE_CLI "$REGISTRY_HOST_PREFIX""$REGISTRY_NAMESPACE"/sample-tests:"$TEST_UUID"
	fi

	print_status "* Deleting namespace $NAMESPACE"
	kubectl delete namespace "$NAMESPACE"

	cd "$ROOT/../staging"
#	kustomize build config/crd | kubectl delete -f -
#	kubectl delete -f config/crd/basesapi.kouplet.com_koupletbuilds_crd.yaml
#	kubectl delete -f deploy/crds/api.kouplet.com_kouplettests_crd.yaml
}

set -eo pipefail
trap dispose EXIT

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# -----------------------------------------------------
# Calculate based on input fields

TEST_UUID=kouplet-`head /dev/urandom | tr -dc a-z0-9 | head -c 13 ; echo ''`
CONTAINER_LABEL="test-$TEST_UUID"
CONTAINER_IMAGE="$REGISTRY_HOST_PREFIX""$REGISTRY_NAMESPACE""/$CONTAINER_REPO_NAME:$CONTAINER_LABEL"


#NAMESPACE="$TEST_UUID"
NAMESPACE=kouplet

# -----------------------------------------------------
print_status "* Build and push the builder image"

cd "$ROOT/../builder-image"


CONTAINER_REGISTRY_NAMESPACE=jgwest \
	CONTAINER_IMAGE=kouplet-builder-util-test \
	CONTAINER_REGISTRY_PREFIX="$REGISTRY_HOST_PREFIX" \
	CONTAINER_IMAGE_LABEL="$TEST_UUID" \
	make build-image-push


#docker build -t kouplet-builder-util .
#docker tag kouplet-builder-util "$REGISTRY_HOST_PREFIX""$REGISTRY_NAMESPACE"/kouplet-builder-util-test:"$TEST_UUID"

#docker push "$REGISTRY_HOST_PREFIX""$REGISTRY_NAMESPACE"/kouplet-builder-util-test:"$TEST_UUID"


# -----------------------------------------------------
print_status "* Build and push the operator"

cd "$ROOT/../staging"
make

echo ci is "$CONTAINER_IMAGE"

IMG="$CONTAINER_IMAGE" make docker-build 
IMG="$CONTAINER_IMAGE" make docker-push
#operator-sdk build "$CONTAINER_IMAGE"
#docker push "$CONTAINER_IMAGE"

# -----------------------------------------------------
print_status "* Create namespace '$NAMESPACE' and create secrets"

kubectl create namespace "$NAMESPACE"

if [ -n "$IMAGE_PULL_SECRET_YAML_PATH" ]; then
	kubectl apply -n "$NAMESPACE" -f "$IMAGE_PULL_SECRET_YAML_PATH"
fi


kubectl create secret generic kouplet-ssh-key-secret -n "$NAMESPACE" \
	--from-file=ssh-privatekey="$SSH_KEY_PATH/id_rsa" \
	--from-file=ssh-publickey="$SSH_KEY_PATH/id_rsa.pub"


if [ -n "$REGISTRY_HOST" ]; then
	kubectl create secret generic kouplet-container-registry-secret -n "$NAMESPACE" \
		--from-file=username="$CONTAINER_REGISTRY_USERNAME_PATH" \
		--from-file=password="$CONTAINER_REGISTRY_PASSWORD_PATH" \
		--from-literal=registry="$REGISTRY_HOST"
else
	kubectl create secret generic kouplet-container-registry-secret -n "$NAMESPACE" \
		--from-file=username="$CONTAINER_REGISTRY_USERNAME_PATH" \
		--from-file=password="$CONTAINER_REGISTRY_PASSWORD_PATH"
fi


kubectl create secret generic kouplet-s3-secret -n "$NAMESPACE" \
	--from-file=accessKeyID="$S3_ACCESS_KEY_ID_PATH" \
	--from-file=secretAccessKey="$S3_SECRET_ACCESS_KEY_PATH"

# -----------------------------------------------------
print_status "* Apply k8s operator resources"

cd "$ROOT/../staging"

kubectl apply -n "$NAMESPACE" -f "$ROOT/../../pvc.yaml"

TARGET_NAMESPACE="$NAMESPACE" IMG="$CONTAINER_IMAGE" make deploy


# ------------------------

# TEMP_OPERATOR_YAML=`mktemp`

# kubectl apply -n "$NAMESPACE" -f deploy/crds/api.kouplet.com_koupletbuilds_crd.yaml
# kubectl apply -n "$NAMESPACE" -f deploy/crds/api.kouplet.com_kouplettests_crd.yaml

# kubectl apply -n "$NAMESPACE" -f "$ROOT/../../pvc.yaml"

# cp deploy/operator.yaml "$TEMP_OPERATOR_YAML"
# sed -i 's|REPLACE_IMAGE|'$CONTAINER_IMAGE'|g' "$TEMP_OPERATOR_YAML"

# kubectl create -n "$NAMESPACE" -f deploy/service_account.yaml

if [ -n "$IMAGE_PULL_SECRET_NAME" ]; then
	echo "* Patching serviceaccount"
	kubectl patch serviceaccount kouplet-operator-controller-manager -p '{"imagePullSecrets": [{"name": "'$IMAGE_PULL_SECRET_NAME'"}]}' -n "$NAMESPACE"
	kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "'$IMAGE_PULL_SECRET_NAME'"}]}' -n "$NAMESPACE"
	
	kubectl delete pods --all
fi

# kubectl create -n "$NAMESPACE" -f deploy/role.yaml
# kubectl create -n "$NAMESPACE" -f deploy/role_binding.yaml
# kubectl create -n "$NAMESPACE" -f "$TEMP_OPERATOR_YAML"

# -----------------------------------------------------
print_status "* Create KoupletBuild"

cd "$ROOT/../ci-resources"

KOUPLET_BUILD_YAML=`mktemp`
cp test-koupletbuild.yaml  "$KOUPLET_BUILD_YAML"
sed -i 's|my-image-tag|'$TEST_UUID'|g' "$KOUPLET_BUILD_YAML"

sed -i 's|my-builder-image-tag|'$TEST_UUID'|g' "$KOUPLET_BUILD_YAML"

kubectl create -n "$NAMESPACE" -f "$KOUPLET_BUILD_YAML"

rm -f "$KOUPLET_BUILD_YAML"

# -----------------------------------------------------
print_status "* Wait for KoupletBuild to complete"

BUILD_SUCCESS=false

while true; do

	export BUILD_COMPLETED=`kubectl -n "$NAMESPACE" get koupletbuild/sample-tests -o=jsonpath='{.status.status}'`

	if [[ "$BUILD_COMPLETED" == "Completed" ]]; then
		print_status "* Build complete"
		set +euo pipefail
		kubectl -n "$NAMESPACE" get koupletbuild/sample-tests -o=json | grep -q "\"jobSucceeded\": true"
		RC=$?
		set -euo pipefail
		
		if [ "$RC" -eq "0" ]; then
			BUILD_SUCCESS=true
		fi
		break

	fi

	sleep 5s	
done

echo $BUILD_SUCCESS

if [[ "$BUILD_SUCCESS" == "false" ]]; then
	exit 1
fi


# -----------------------------------------------------
print_status "* Create KoupletTest"

cd "$ROOT/../ci-resources"

KOUPLET_TEST_YAML=`mktemp`
cp test-kouplettest.yaml  "$KOUPLET_TEST_YAML"
sed -i 's|my-image-tag|'$TEST_UUID'|g' "$KOUPLET_TEST_YAML"

kubectl create -n "$NAMESPACE" -f "$KOUPLET_TEST_YAML"

rm -f "$KOUPLET_TEST_YAML"

# -----------------------------------------------------
print_status "* Wait for KoupletTest to complete"

TEST_SUCCESS=false

while true; do

	export TEST_COMPLETED=`kubectl -n "$NAMESPACE" get kouplettest/sample-tests-run  -o=jsonpath='{.status.status}'`

	if [[ "$TEST_COMPLETED" == "Complete" ]]; then
		print_status "* kouplettest complete"
		set +euo pipefail
		kubectl -n "$NAMESPACE" get kouplettest/sample-tests-run -o=jsonpath='{.status.percent}' | grep -q "100"
		RC=$?
		set -euo pipefail
		
		if [ "$RC" -eq "0" ]; then
			TEST_SUCCESS=true
		fi
		break

	fi

	sleep 5s	
done

if [[ "$TEST_SUCCESS" == "false" ]]; then
	print_status "* Test failed"
	exit 1
fi

# -------------------------------------------

S3_OBJECT_LIST=`kubectl get kouplettest/sample-tests-run -o=jsonpath='{range .status.results[*]}{.log}{"\n"}'`

S3_OBJECT_CONTENTS_PATH=`mktemp`
S3_OBJECT_CONTENTS=`mktemp`
echo "$S3_OBJECT_LIST" > "$S3_OBJECT_CONTENTS"

while IFS="" read -r p || [ -n "$p" ]
do
	S3_OBJECT_KEY="$p"

	aws --endpoint-url "$S3_ENDPOINT_URL" --region="$S3_REGION"  s3 cp s3://kouplet-bucket/$S3_OBJECT_KEY "$S3_OBJECT_CONTENTS_PATH"

	set +euo pipefail
	cat "$S3_OBJECT_CONTENTS_PATH" | grep -q "testcase classname"
	RC=$?
	set -euo pipefail
	if [ "$RC" -ne "0" ]; then
		print_status "* Test failed: could not locate test case xml"
		exit 1
	fi

	set +euo pipefail
	cat "$S3_OBJECT_CONTENTS_PATH" | grep -q "Tests run: 5"
	RC=$?
	set -euo pipefail
	if [ "$RC" -ne "0" ]; then
		print_status "* Test failed: could not locate maven test string"
		exit 1
	fi

	aws --endpoint-url "$S3_ENDPOINT_URL" --region="$S3_REGION"  s3 rm s3://kouplet-bucket/$S3_OBJECT_KEY

done < "$S3_OBJECT_CONTENTS"

# -------------------------------------------

S3_OBJECT_LIST=`kubectl get kouplettest/sample-tests-run -o=jsonpath='{range .status.results[*]}{.result}{"\n"}'`

S3_OBJECT_CONTENTS_PATH=`mktemp`
S3_OBJECT_CONTENTS=`mktemp`
echo "$S3_OBJECT_LIST" > "$S3_OBJECT_CONTENTS"


while IFS="" read -r p || [ -n "$p" ]
do
	S3_OBJECT_KEY="$p"

	aws --endpoint-url "$S3_ENDPOINT_URL" --region="$S3_REGION"  s3 cp s3://kouplet-bucket/$S3_OBJECT_KEY "$S3_OBJECT_CONTENTS_PATH"

	set +euo pipefail
	cat "$S3_OBJECT_CONTENTS_PATH" | grep -q "testcase classname"
	RC=$?
	set -euo pipefail
	if [ "$RC" -ne "0" ]; then
		print_status "* Test failed: could not locate test case xml"
		exit 1
	fi

	aws --endpoint-url "$S3_ENDPOINT_URL" --region="$S3_REGION"  s3 rm s3://kouplet-bucket/$S3_OBJECT_KEY

done < "$S3_OBJECT_CONTENTS"

print_status "* Test passed."

# -----------------------------------------------------
print_status "* Test complete, calling dispose"
# Dispose is called by the exit trap

