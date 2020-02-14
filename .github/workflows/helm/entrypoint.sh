#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

VERSION=$(git describe --tags --dirty | cut -c 2-)
echo "Publishing version '$VERSION'"

helm lint helm-chart/rode
helm registry login -u $GITHUB_ACTOR -p $INPUT_GITHUB_TOKEN docker.pkg.github.com 


sed -i "s/version:.*/version: $VERSION/" helm-chart/rode/Chart.yaml
sed -i "s/appVersion:.*/appVersion: v$VERSION/" helm-chart/rode/Chart.yaml
helm chart save helm-chart/rode docker.pkg.github.com/liatrio/rode/chart:$VERSION
helm chart push docker.pkg.github.com/liatrio/rode/chart:$VERSION
