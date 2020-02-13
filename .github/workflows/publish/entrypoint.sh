#!/bin/sh 
set -e

SKAFFOLD_DEFAULT_REPO=docker.pkg.github.com/liatrio/rode
VERSION=$(git describe --tags --dirty | cut -c 2-)

echo "Publishing $VERSION"

skaffold build

helm lint helm-chart/rode
helm package --version $VERSION --app-version $VERSION helm-chart/rode
#curl -f -X PUT -u $ARTIFACTORY_CREDS -T rode-$VERSION.tgz $(HELM_REPOSITORY)/rode-$VERSION.tgz
