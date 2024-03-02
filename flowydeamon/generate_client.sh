#!/bin/sh

cd api
openapi-generator-cli generate -g go -i ../../flowy_api.json --additional-properties packageName=api
rm -rf .openapi-generator README.md docs/ test/ go.mod git_push.sh

