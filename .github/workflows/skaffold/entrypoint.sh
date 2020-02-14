#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

docker login docker.pkg.github.com --username $GITHUB_ACTOR --password $INPUT_GITHUB_TOKEN
skaffold build --default-repo docker.pkg.github.com/liatrio/rode
