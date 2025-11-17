#!/bin/bash

set -eu

eval $(TASKCLUSTER_ROOT_URL=https://firefox-ci-tc.services.mozilla.com taskcluster signin)

unset FXCI_TASKCLUSTER_CERTIFICATE
export FXCI_TASKCLUSTER_CLIENT_ID=${TASKCLUSTER_CLIENT_ID}
export FXCI_TASKCLUSTER_ACCESS_TOKEN=${TASKCLUSTER_ACCESS_TOKEN}

unset TASKCLUSTER_CERTIFICATE
unset TASKCLUSTER_CLIENT_ID
unset TASKCLUSTER_ACCESS_TOKEN

unset COMMUNITY_TASKCLUSTER_CERTIFICATE
export COMMUNITY_TASKCLUSTER_CLIENT_ID=static/taskcluster/root
export COMMUNITY_TASKCLUSTER_ACCESS_TOKEN="$(pass community-tc/root | head -1)"

unset DEV_TASKCLUSTER_CERTIFICATE
export DEV_TASKCLUSTER_CLIENT_ID=static/taskcluster/root
export DEV_TASKCLUSTER_ACCESS_TOKEN="$(pass dev.alpha.taskcluster-dev.net/root | head -1)"

go install

tc-staging-sync
