#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

VERSION=$(git describe --tags --dirty | cut -c 2-)
echo "Publishing version '$VERSION'"

helm lint helm-chart/rode
#helm registry login -u $GITHUB_ACTOR -p $INPUT_GITHUB_TOKEN docker.pkg.github.com 
helm package --version $VERSION --app-version $VERSION helm-chart/rode
