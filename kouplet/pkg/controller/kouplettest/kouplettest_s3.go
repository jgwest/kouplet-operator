package kouplettest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	apiv1alpha1 "github.com/jgwest/kouplet/pkg/apis/api/v1alpha1"
	"github.com/jgwest/kouplet/pkg/controller/shared"
	"github.com/minio/minio-go/v6"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func uploadLogFromKubeCreds(logContents string, objectName string, kos apiv1alpha1.KoupletObjectStorage, ctx KTContext) error {

	accessKeyIDDecoded, secretAccessKeyIDDecoded, err := getSecretData(kos, ctx)
	if err != nil {
		return err
	}

	return uploadLogToObjectStorage(logContents, objectName, kos.BucketName, kos.BucketLocation, kos.Endpoint, accessKeyIDDecoded, secretAccessKeyIDDecoded)
}

func uploadLogToObjectStorage(logContents string, objectName string, bucketName string, bucketLocation string, endpoint string, accessKeyID string, secretAccessKey string) error {

	useSSL := true

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		shared.LogErrorMsg(err, "Error on new minio client")
		return err
	}

	err = minioClient.MakeBucket(bucketName, bucketLocation)
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(bucketName)
		if errBucketExists == nil && exists {
			// log.Info("We already own " + bucketName)
		} else {
			shared.LogErrorMsg(err, "MakeBacket error")
			return err
		}
	} else {
		shared.LogInfo("Successfully created " + bucketName)
	}

	// Upload the string with PutObject
	_, err = minioClient.PutObject(bucketName, objectName, strings.NewReader(logContents), -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		shared.LogErrorMsg(err, "Error on put object")
		return err
	}

	shared.LogInfo("Successfully uploaded " + objectName)

	return nil
}

func getSecretData(kos apiv1alpha1.KoupletObjectStorage, ctx KTContext) (string, string, error) {
	secret := corev1.Secret{}

	err := (*ctx.client).Get(context.TODO(), types.NamespacedName{
		Namespace: ctx.namespace,
		Name:      kos.CredentialsSecretName,
	}, &secret)

	if err != nil {
		shared.LogErrorMsg(err, "Unable to retrieve credentials object '"+kos.CredentialsSecretName+"'")
		return "", "", err
	}

	val, err := json.Marshal(secret.Data)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to marshall secret data")
		return "", "", err
	}

	secretData := map[string]string{}

	err = json.Unmarshal(val, &secretData)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to unmarshal secret data")
		return "", "", err
	}

	accessKeyIDBase64 := secretData["accessKeyID"]
	if len(string(accessKeyIDBase64)) == 0 {
		return "", "", fmt.Errorf("Secret '%s' did not contain accessKeyID field", kos.CredentialsSecretName)
	}
	accessKeyIDDecoded, err := base64.StdEncoding.DecodeString(accessKeyIDBase64)
	if err != nil {
		shared.LogErrorMsg(err, "Unable to decode access key ID")
		return "", "", err
	}

	secretAccessKeyBase64 := secretData["secretAccessKey"]
	if len(string(secretAccessKeyBase64)) == 0 {
		return "", "", fmt.Errorf("Secret '%s' did not contain secretAccessKey field", kos.CredentialsSecretName)
	}
	secretAccessKeyIDDecoded, err := base64.StdEncoding.DecodeString(string(secretAccessKeyBase64))
	if err != nil {
		shared.LogErrorMsg(err, "Unable to decode secret access key ID")
		return "", "", err
	}

	return string(accessKeyIDDecoded), string(secretAccessKeyIDDecoded), nil
}
