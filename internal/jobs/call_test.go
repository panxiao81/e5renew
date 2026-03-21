package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/DATA-DOG/go-sqlmock"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type fakeTokenCredential struct{}

func (fakeTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestGetUsersAndMessagesClientScope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	originalCredentialFactory := newClientSecretCredential
	originalClientFactory := newGraphServiceClientWithCredentials
	originalUsersGetter := getGraphUsers
	originalTenant := viper.GetString("azureAD.tenant")
	originalClientID := viper.GetString("azureAD.clientID")
	originalClientSecret := viper.GetString("azureAD.clientSecret")
	t.Cleanup(func() {
		newClientSecretCredential = originalCredentialFactory
		newGraphServiceClientWithCredentials = originalClientFactory
		getGraphUsers = originalUsersGetter
		viper.Set("azureAD.tenant", originalTenant)
		viper.Set("azureAD.clientID", originalClientID)
		viper.Set("azureAD.clientSecret", originalClientSecret)
	})

	viper.Set("azureAD.tenant", "tenant")
	viper.Set("azureAD.clientID", "client-id")
	viper.Set("azureAD.clientSecret", "client-secret")

	t.Run("returns credential creation error", func(t *testing.T) {
		apiSvc, _, _, cleanup := makeServices(t)
		defer cleanup()

		newClientSecretCredential = func(tenantID, clientID, clientSecret string) (azcore.TokenCredential, error) {
			return nil, errors.New("bad credential config")
		}

		counts, err := GetUsersAndMessagesClientScope(context.Background(), apiSvc, logger)
		require.Nil(t, counts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create Azure AD client secret credentials")
	})

	t.Run("returns graph client creation error", func(t *testing.T) {
		apiSvc, _, _, cleanup := makeServices(t)
		defer cleanup()

		newClientSecretCredential = func(tenantID, clientID, clientSecret string) (azcore.TokenCredential, error) {
			return fakeTokenCredential{}, nil
		}
		newGraphServiceClientWithCredentials = func(credential azcore.TokenCredential, scopes []string) (*msgraphsdkgo.GraphServiceClient, error) {
			return nil, errors.New("client init failed")
		}

		counts, err := GetUsersAndMessagesClientScope(context.Background(), apiSvc, logger)
		require.Nil(t, counts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create Microsoft Graph client")
	})

	t.Run("logs failed graph users call", func(t *testing.T) {
		apiSvc, _, mock, cleanup := makeServices(t)
		defer cleanup()

		mock.ExpectExec(`(?is)insert into api_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

		newClientSecretCredential = func(tenantID, clientID, clientSecret string) (azcore.TokenCredential, error) {
			return fakeTokenCredential{}, nil
		}
		newGraphServiceClientWithCredentials = func(credential azcore.TokenCredential, scopes []string) (*msgraphsdkgo.GraphServiceClient, error) {
			return &msgraphsdkgo.GraphServiceClient{}, nil
		}
		getGraphUsers = func(ctx context.Context, client *msgraphsdkgo.GraphServiceClient) (models.UserCollectionResponseable, error) {
			return nil, errors.New("graph unavailable")
		}

		counts, err := GetUsersAndMessagesClientScope(context.Background(), apiSvc, logger)
		require.Nil(t, counts)
		require.EqualError(t, err, "graph unavailable")
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("returns processed user message counts", func(t *testing.T) {
		apiSvc, _, mock, cleanup := makeServices(t)
		defer cleanup()

		mock.ExpectExec(`(?is)insert into api_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

		newClientSecretCredential = func(tenantID, clientID, clientSecret string) (azcore.TokenCredential, error) {
			return fakeTokenCredential{}, nil
		}
		newGraphServiceClientWithCredentials = func(credential azcore.TokenCredential, scopes []string) (*msgraphsdkgo.GraphServiceClient, error) {
			return &msgraphsdkgo.GraphServiceClient{}, nil
		}
		getGraphUsers = func(ctx context.Context, client *msgraphsdkgo.GraphServiceClient) (models.UserCollectionResponseable, error) {
			alice := models.NewUser()
			aliceName := "Alice"
			alice.SetDisplayName(&aliceName)
			alice.SetMessages([]models.Messageable{models.NewMessage(), models.NewMessage()})

			bob := models.NewUser()
			bobName := "Bob"
			bob.SetDisplayName(&bobName)
			bob.SetMessages([]models.Messageable{models.NewMessage()})

			resp := models.NewUserCollectionResponse()
			resp.SetValue([]models.Userable{alice, bob})
			return resp, nil
		}

		counts, err := GetUsersAndMessagesClientScope(context.Background(), apiSvc, logger)
		require.NoError(t, err)
		require.Equal(t, map[string]int{"Alice": 2, "Bob": 1}, counts)
		require.Eventually(t, func() bool {
			return mock.ExpectationsWereMet() == nil
		}, time.Second, 10*time.Millisecond)
	})
}
