# tc-dev-sync
This utility syncs the state of [Community-TC](https://community-tc.services.mozilla.com) and [FirefoxCI](https://firefox-ci-tc.services.mozilla.com) to the [dev](https://dev.alpha.taskcluster-dev.net) taskcluster environment for:

* Clients
* Roles
* Worker Pools
* Secrets

Furthermore, it triggers a Community-TC decision task for the taskcluster monorepo CI, to validate that tasks resolve successfully and match those that run in Community-TC, and similarly creates a decision task for a FirefoxCI try push.

# Running

Simply run `./run.sh`.
