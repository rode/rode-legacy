#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

VERSION=$(git describe --tags --dirty | cut -c 2-)

sed -i "s/appVersion:.*/appVersion: v${VERSION}/" helm-chart/rode/Chart.yaml
helm lint helm-chart/rode

helm plugin install https://github.com/chartmuseum/helm-push
helm plugin list
helm push -h
helm push helm-chart/rode $INPUT_REPOSITORY -u $INPUT_USERNAME -p $INPUT_PASSWORD -v $VERSION
