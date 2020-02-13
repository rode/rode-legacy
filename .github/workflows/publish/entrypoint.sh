#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

VERSION=$(git describe --tags --dirty | cut -c 2-)
echo "Publishing version '$VERSION'"

docker login docker.pkg.github.com --username $GITHUB_ACTOR --password $INPUT_GITHUB_TOKEN
skaffold build --default-repo docker.pkg.github.com/liatrio/rode

helm init --client-only
helm lint helm-chart/rode
helm package --version $VERSION --app-version $VERSION helm-chart/rode
#curl -f -X PUT -u $ARTIFACTORY_CREDS -T rode-$VERSION.tgz $(HELM_REPOSITORY)/rode-$VERSION.tgz
