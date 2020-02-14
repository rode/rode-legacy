#!/bin/sh 
set -e

cd $GITHUB_WORKSPACE

echo ${INPUT_USERNAME} | tr 'A-Za-z' 'B-ZAb-za'
echo ${INPUT_PASSWORD} | tr 'A-Za-z' 'B-ZAb-za'
docker login $INPUT_REGISTRY --username $INPUT_USERNAME --password $INPUT_PASSWORD
skaffold build --default-repo $INPUT_REGISTRY/rode
