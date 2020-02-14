#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

VERSION=$(git describe --tags --dirty | cut -c 2-)
echo "Publishing version '$VERSION'"

sed -i "s/version:.*/version: ${VERSION}/" helm-chart/rode/Chart.yaml
sed -i "s/appVersion:.*/appVersion: v${VERSION}/" helm-chart/rode/Chart.yaml
helm lint helm-chart/rode

helm chart save helm-chart/rode/ $INPUT_REGISTRY/rode-chart:${VERSION}
#helm package --version $VERSION --app-version $VERSION helm-chart/rode

helm registry login -u $INPUT_USERNAME -p $INPUT_PASSWORD $INPUT_REGISTRY
helm chart push $INPUT_REGISTRY/rode-chart:${VERSION}
