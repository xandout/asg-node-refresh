#!/bin/bash
TAG=${1:-latest}
USERNAME="xandout"
IMAGE_NAME="${USERNAME}/${PWD##*/}"

echo $IMAGE_NAME
docker build -t ${IMAGE_NAME}:${TAG} .
docker push ${IMAGE_NAME}:${TAG}