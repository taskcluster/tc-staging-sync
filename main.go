package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	tcclient "github.com/taskcluster/taskcluster/v93/clients/client-go"
	"github.com/taskcluster/taskcluster/v93/clients/client-go/tcauth"
	"github.com/taskcluster/taskcluster/v93/clients/client-go/tcsecrets"
	"github.com/taskcluster/taskcluster/v93/clients/client-go/tcworkermanager"
	"github.com/taskcluster/tc-staging-sync/workerpool"
)

const (
	communityRootURL = "https://community-tc.services.mozilla.com"
	fxciRootURL      = "https://firefox-ci-tc.services.mozilla.com"
	devRootURL       = "https://dev.alpha.taskcluster-dev.net"
)

type ProviderIDMutator func(sourceProviderID string) (targetProviderID string)

func fxciProviderIDMutator(sourceProviderID string) (targetProviderID string) {
	switch sourceProviderID {
	case "aws":
		return "aws"
	case "azure2":
		return "azure"
	case "azure_trusted":
		return "azure"
	case "fxci-level1-gcp":
		return "google"
	case "fxci-level3-gcp":
		return "google"
	case "fxci-test-gcp":
		return "google"
	case "fxci-translations-sandbox-gcp":
		return "google"
	case "static":
		return "static"
	default:
		panic("Unknown Provider ID " + sourceProviderID)
	}
}

func communityProviderIDMutator(sourceProviderID string) (targetProviderID string) {
	switch sourceProviderID {
	case "community-tc-workers-aws":
		return "aws"
	case "community-tc-workers-azure":
		return "azure"
	case "community-tc-workers-google":
		return "google"
	case "static":
		return "static"
	default:
		panic("Unknown Provider ID " + sourceProviderID)
	}
}

var (
	communityCreds *tcclient.Credentials = CredentialsFromEnvVars("COMMUNITY_")
	fxciCreds      *tcclient.Credentials = CredentialsFromEnvVars("FXCI_")
	devCreds       *tcclient.Credentials = CredentialsFromEnvVars("DEV_")

	communityAuth          *tcauth.Auth                   = tcauth.New(communityCreds, communityRootURL)
	communitySecrets       *tcsecrets.Secrets             = tcsecrets.New(communityCreds, communityRootURL)
	communityWorkerManager *tcworkermanager.WorkerManager = tcworkermanager.New(communityCreds, communityRootURL)

	fxciAuth          *tcauth.Auth                   = tcauth.New(fxciCreds, fxciRootURL)
	fxciSecrets       *tcsecrets.Secrets             = tcsecrets.New(fxciCreds, fxciRootURL)
	fxciWorkerManager *tcworkermanager.WorkerManager = tcworkermanager.New(fxciCreds, fxciRootURL)

	devAuth          *tcauth.Auth                   = tcauth.New(devCreds, devRootURL)
	devSecrets       *tcsecrets.Secrets             = tcsecrets.New(devCreds, devRootURL)
	devWorkerManager *tcworkermanager.WorkerManager = tcworkermanager.New(devCreds, devRootURL)
)

// CredentialsFromEnvVars creates and returns Taskcluster credentials
// initialised from the values of environment variables:
//
//	<prefix>TASKCLUSTER_CLIENT_ID
//	<prefix>TASKCLUSTER_ACCESS_TOKEN
//	<prefix>TASKCLUSTER_CERTIFICATE
//
// No validation is performed on the assigned values, and unset environment
// variables will result in empty string values.
func CredentialsFromEnvVars(prefix string) *tcclient.Credentials {
	return &tcclient.Credentials{
		ClientID:    os.Getenv(prefix + "TASKCLUSTER_CLIENT_ID"),
		AccessToken: os.Getenv(prefix + "TASKCLUSTER_ACCESS_TOKEN"),
		Certificate: os.Getenv(prefix + "TASKCLUSTER_CERTIFICATE"),
	}
}

func main() {
	wp := workerpool.New(100)
	wp.AddWork(CopyRoles(communityAuth))
	wp.AddWork(CopyRoles(fxciAuth))
	wp.AddWork(CopyClients(communityAuth))
	wp.AddWork(CopyClients(fxciAuth))
	wp.AddWork(CopyWorkerPools(communityWorkerManager, communityProviderIDMutator))
	wp.AddWork(CopyWorkerPools(fxciWorkerManager, fxciProviderIDMutator))
	wp.AddWork(CopySecrets(communitySecrets))
	wp.AddWork(CopySecrets(fxciSecrets))
	wp.Done()
	wp.OnComplete(func(result workerpool.Result) {
		log.Printf("%s", result)
	})

	// Handle secret project/taskcluster/testing/client-libraries for client project/taskcluster/testing/client-libraries
	s, err := devSecrets.Get("project/taskcluster/testing/client-libraries")
	if err != nil {
		panic(err)
	}
	var data map[string]any
	err = json.Unmarshal(s.Secret, &data)
	if err != nil {
		panic(err)
	}
	ccr, err := devAuth.ResetAccessToken("project/taskcluster/testing/client-libraries")
	if err != nil {
		panic(err)
	}
	data["TASKCLUSTER_ACCESS_TOKEN"] = ccr.AccessToken
	s.Secret, err = json.Marshal(data)
	if err != nil {
		panic(err)
	}
	err = devSecrets.Set("project/taskcluster/testing/client-libraries", s)
	if err != nil {
		panic(err)
	}

	// Handle secret project/taskcluster/testing/generic-worker/ci-creds for client project/taskcluster/testing/client-libraries
	s, err = devSecrets.Get("project/taskcluster/testing/generic-worker/ci-creds")
	if err != nil {
		panic(err)
	}
	data = map[string]any{}
	err = json.Unmarshal(s.Secret, &data)
	if err != nil {
		panic(err)
	}
	ccr, err = devAuth.ResetAccessToken("project/taskcluster/testing/client-libraries")
	if err != nil {
		panic(err)
	}
	re := regexp.MustCompile(`(?m)TASKCLUSTER_ACCESS_TOKEN=.*$`)
	for _, key := range []string{
		"b64_encoded_credentials_script",
		"b64_encoded_credentials_batch_script",
	} {
		dec, err := base64.StdEncoding.DecodeString(data[key].(string))
		if err != nil {
			panic(fmt.Errorf("base64 decode failed: %w", err))
		}
		replaced := re.ReplaceAllString(string(dec), "TASKCLUSTER_ACCESS_TOKEN="+ccr.AccessToken)
		data[key] = base64.StdEncoding.EncodeToString([]byte(replaced))
	}
	s.Secret, err = json.Marshal(data)
	if err != nil {
		panic(err)
	}
	err = devSecrets.Set("project/taskcluster/testing/client-libraries", s)
	if err != nil {
		panic(err)
	}
}

func CopyRoles(auth *tcauth.Auth) workerpool.WorkSubmitter {
	return func(context *workerpool.SubmitterContext) {
		continuationToken := ""
		for {
			roles, err := auth.ListRoles2(continuationToken, "")
			if err != nil {
				panic(err)
			}
			for _, role := range roles.Roles {
				if role.RoleID == "anonymous" {
					continue
				}
				context.RequestChannel <- CopyRole(
					role,
					devAuth,
				)

			}
			continuationToken = roles.ContinuationToken
			if continuationToken == "" {
				break
			}
		}
	}
}

func CopyClients(auth *tcauth.Auth) workerpool.WorkSubmitter {
	return func(context *workerpool.SubmitterContext) {
		continuationToken := ""
		for {
			clients, err := auth.ListClients(continuationToken, "", "")
			if err != nil {
				panic(err)
			}
			for _, client := range clients.Clients {
				if strings.HasPrefix(client.ClientID, "static/") {
					continue
				}
				context.RequestChannel <- CopyClient(
					client,
					devAuth,
				)
			}
			continuationToken = clients.ContinuationToken
			if continuationToken == "" {
				break
			}
		}
	}
}

func CopySecrets(secrets *tcsecrets.Secrets) workerpool.WorkSubmitter {
	return func(context *workerpool.SubmitterContext) {
		continuationToken := ""
		for {
			secretsList, err := secrets.List(continuationToken, "")
			if err != nil {
				panic(err)
			}
			for _, secretName := range secretsList.Secrets {
				context.RequestChannel <- func(secretName string) workerpool.Work {
					return func(workerId int) workerpool.Result {
						secret, err := secrets.Get(secretName)
						if err != nil {
							panic(err)
						}
						return CopySecret(
							secretName,
							secret,
							devSecrets,
						)(workerId)
					}
				}(secretName)
			}
			continuationToken = secretsList.ContinuationToken
			if continuationToken == "" {
				break
			}
		}
	}
}

func CopyWorkerPools(workerManager *tcworkermanager.WorkerManager, mut ProviderIDMutator) workerpool.WorkSubmitter {
	return func(context *workerpool.SubmitterContext) {
		continuationToken := ""
		for {
			workerPools, err := workerManager.ListWorkerPools(continuationToken, "")
			if err != nil {
				panic(err)
			}
			for _, workerPool := range workerPools.WorkerPools {
				context.RequestChannel <- CopyWorkerPool(
					workerPool,
					mut,
					devWorkerManager,
				)
			}
			continuationToken = workerPools.ContinuationToken
			if continuationToken == "" {
				break
			}
		}
	}
}

func CopyRole(role tcauth.GetRoleResponse, auth *tcauth.Auth) workerpool.Work {
	return func(workerId int) workerpool.Result {
		_, err := auth.CreateRole(
			role.RoleID,
			&tcauth.CreateRoleRequest{
				Description: role.Description,
				Scopes:      role.Scopes,
			},
		)
		if err != nil {
			_, err = auth.UpdateRole(
				role.RoleID,
				&tcauth.CreateRoleRequest{
					Description: role.Description,
					Scopes:      role.Scopes,
				},
			)
		}
		if err != nil {
			panic(fmt.Sprintf("Could not create role %v: %v", role.RoleID, err))
		}
		return fmt.Sprintf("Copied role %v", role.RoleID)
	}
}

func CopyClient(client tcauth.GetClientResponse, auth *tcauth.Auth) workerpool.Work {
	return func(workerId int) workerpool.Result {
		_, err := auth.CreateClient(
			client.ClientID,
			&tcauth.CreateClientRequest{
				DeleteOnExpiration: client.DeleteOnExpiration,
				Description:        client.Description,
				Expires:            client.Expires,
				Scopes:             client.Scopes,
			},
		)
		if err != nil {
			_, err = auth.UpdateClient(
				client.ClientID,
				&tcauth.CreateClientRequest{
					DeleteOnExpiration: client.DeleteOnExpiration,
					Description:        client.Description,
					Expires:            client.Expires,
					Scopes:             client.Scopes,
				},
			)
		}
		if err != nil {
			panic(fmt.Sprintf("Could not create client %v: %v", client.ClientID, err))
		}
		return fmt.Sprintf("Copied client %v", client.ClientID)
	}
}

func CopySecret(secretName string, secret *tcsecrets.Secret, secrets *tcsecrets.Secrets) workerpool.Work {
	return func(workerId int) workerpool.Result {
		err := secrets.Set(
			secretName,
			secret,
		)
		if err != nil {
			panic(fmt.Sprintf("Could not create secfet %v: %v", secretName, err))
		}
		return fmt.Sprintf("Copied secret %v", secretName)
	}
}

func CopyWorkerPool(wp tcworkermanager.WorkerPoolFullDefinition, mut ProviderIDMutator, workermanager *tcworkermanager.WorkerManager) workerpool.Work {
	return func(workerId int) workerpool.Result {
		providerID := mut(wp.ProviderID)
		if providerID == "aws" || providerID == "static" {
			return fmt.Sprintf("Skipping %v worker pool %v", providerID, wp.WorkerPoolID)
		}

		var config map[string]any

		err := json.Unmarshal(wp.Config, &config)
		if err != nil {
			panic(fmt.Errorf("parsing wp.Config as JSON object failed: %w (payload: %s)", err, string(wp.Config)))
		}
		config["minCapacity"] = 0

		b, err := json.Marshal(config)
		if err != nil {
			panic(fmt.Errorf("re-encoding updated config failed: %w (map: %#v)", err, config))
		}

		wp.Config = json.RawMessage(b)

		_, err = workermanager.CreateWorkerPool(
			wp.WorkerPoolID,
			&tcworkermanager.WorkerPoolDefinition{
				Config:       wp.Config,
				Description:  wp.Description,
				EmailOnError: wp.EmailOnError,
				Owner:        wp.Owner,
				ProviderID:   providerID,
			},
		)
		if err != nil {
			_, err = workermanager.UpdateWorkerPool(
				wp.WorkerPoolID,
				&tcworkermanager.WorkerPoolDefinition1{
					Config:       wp.Config,
					Description:  wp.Description,
					EmailOnError: wp.EmailOnError,
					Owner:        wp.Owner,
					ProviderID:   providerID,
				},
			)
		}
		if err != nil {
			panic(fmt.Sprintf("Could not create worker pool %v: %v", wp.WorkerPoolID, err))
		}
		return fmt.Sprintf("Copied worker pool %v", wp.WorkerPoolID)
	}
}
