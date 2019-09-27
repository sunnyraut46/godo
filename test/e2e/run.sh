#!/bin/sh

set -xeu

TEST_ID=$(cat /proc/sys/kernel/random/uuid)
export TEST_ID

go test -timeout=1h -v -count=1 github.com/digitalocean/godo/test/e2e/... | tee logs.txt && \
curl -X POST "http://$E2E_SERVER_ADDR/api/results/$TEST_ID/logs" --data-binary @./logs.txt && \
rm logs.txt
