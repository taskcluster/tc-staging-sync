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

repo="taskcluster/taskcluster"
branch="main"

tc_sha="$(gh api --jq '.sha' "/repos/$repo/commits/$branch")"

details_url="$(
  gh api --jq '.check_runs[]
               | select(.app.slug=="community-tc-integration")
               | select(.name=="Decision Task (github-push)")
               | .details_url' \
    "/repos/$repo/commits/$tc_sha/check-runs?per_page=100"
)"

taskclusterCITaskID="$(
  jq -nr --arg url "$details_url" '
    $url
    | sub("^.*/tasks/"; "")
    | split("?")[0]
    | split("#")[0]
  '
)"

push_id="$(curl -s "https://treeherder.mozilla.org/api/project/try/push/?count=1" \
  | jq -r '.results[0].id')"

tryTaskID="$(curl -s "https://treeherder.mozilla.org/api/project/try/jobs/?push_id=$push_id" \
  | jq -r '.results[]
    | select(.job_type_name | test("Decision Task"))
	| .task_id')"

tc-dev-sync "${taskclusterCITaskID}" "${tryTaskID}"

export TASKCLUSTER_ROOT_URL=https://firefox-ci-tc.services.mozilla.com
export TASKCLUSTER_CLIENT_ID="${FXCI_TASKCLUSTER_CLIENT_ID}"
export TASKCLUSTER_ACCESS_TOKEN="${FXCI_TASKCLUSTER_ACCESS_TOKEN}"
unset TASKCLUSTER_CERTIFICATE
taskcluster api auth listRoles | jq > fxci.roles
taskcluster api auth listClients | jq > fxci.clients

export TASKCLUSTER_ROOT_URL=https://community-tc.services.mozilla.com
export TASKCLUSTER_CLIENT_ID="${COMMUNITY_TASKCLUSTER_CLIENT_ID}"
export TASKCLUSTER_ACCESS_TOKEN="${COMMUNITY_TASKCLUSTER_ACCESS_TOKEN}"
unset TASKCLUSTER_CERTIFICATE
taskcluster api auth listRoles | jq > community.roles
taskcluster api auth listClients | jq > community.clients

export TASKCLUSTER_ROOT_URL=https://dev.alpha.taskcluster-dev.net
export TASKCLUSTER_CLIENT_ID="${DEV_TASKCLUSTER_CLIENT_ID}"
export TASKCLUSTER_ACCESS_TOKEN="${DEV_TASKCLUSTER_ACCESS_TOKEN}"
unset TASKCLUSTER_CERTIFICATE
taskcluster api auth listRoles | jq > dev.roles
taskcluster api auth listClients | jq > dev.clients
