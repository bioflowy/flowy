#!/bin/sh

cd api
openapi-generator-cli generate -g go -i ../flowy_api.yaml --additional-properties packageName=api
rm -rf .openapi-generator README.md docs/ test/ go.mod git_push.sh

