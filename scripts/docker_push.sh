#!/bin/bash
docker login -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD";
docker tag centeredge/shawarma-webhook:${TRAVIS_BUILD_NUMBER} centeredge/shawarma-webhook:${TRAVIS_TAG};
docker tag centeredge/shawarma-webhook:${TRAVIS_BUILD_NUMBER} centeredge/shawarma-webhook:latest;
docker push centeredge/shawarma-webhook:${TRAVIS_TAG};
docker push centeredge/shawarma-webhook:latest;
