package services

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/db"
)

func TestMailHelpers(t *testing.T) {
	v := "hello"
	if getStringValue(&v) != "hello" {
		t.Fatal("getStringValue should dereference pointer")
	}
	if getStringValue(nil) != "" {
		t.Fatal("getStringValue nil should return empty string")
	}

	b := true
	if !getBoolValue(&b) {
		t.Fatal("getBoolValue should dereference pointer")
	}
	if getBoolValue(nil) {
		t.Fatal("getBoolValue nil should return false")
	}
}

func TestMailServiceConvertGraphMessagesToMailResponseNil(t *testing.T) {
	svc := &MailService{}
	resp := svc.convertGraphMessagesToMailResponse(nil)
	if resp == nil || len(resp.Value) != 0 {
		t.Fatalf("expected empty response for nil collection, got %#v", resp)
	}
}

func TestMailServiceConvertGraphMessagesToMailResponseValues(t *testing.T) {
	svc := &MailService{}

	id := "m1"
	subject := "hello"
	isRead := true
	received := time.Date(2026, 3, 10, 1, 0, 0, 0, time.UTC)
	addr := "u@example.com"
	name := "User"

	m := models.NewMessage()
	m.SetId(&id)
	m.SetSubject(&subject)
	m.SetIsRead(&isRead)
	m.SetReceivedDateTime(&received)
	sender := models.NewRecipient()
	email := models.NewEmailAddress()
	email.SetAddress(&addr)
	email.SetName(&name)
	sender.SetEmailAddress(email)
	m.SetSender(sender)

	collection := models.NewMessageCollectionResponse()
	collection.SetValue([]models.Messageable{nil, m})

	resp := svc.convertGraphMessagesToMailResponse(collection)
	require.Len(t, resp.Value, 1)
	require.Equal(t, "m1", resp.Value[0].ID)
	require.Equal(t, received.Format(time.RFC3339), resp.Value[0].ReceivedAt)
	require.Equal(t, "u@example.com", resp.Value[0].From.EmailAddress.Address)
}

func makeMailServiceForErrorPath(t *testing.T) (*MailService, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	viper.Set("encryption.key", "mail-test-key")
	encryption, err := NewEncryptionService()
	require.NoError(t, err)

	userTokens := NewUserTokenService(db.New(sqlDB), &oauth2.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), encryption)
	apiSvc := NewAPILogService(db.New(sqlDB), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := NewMailService(userTokens, apiSvc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return svc, mock, cleanup
}

func TestMailServiceLogGraphAPICallAsync(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	apiSvc := NewAPILogService(db.New(sqlDB), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := &MailService{
		apiLogService: apiSvc,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	start := time.Now().Add(-100 * time.Millisecond)
	end := time.Now()
	mock.ExpectExec(`(?is)insert\s+into\s+api_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

	svc.logGraphAPICall(context.Background(), "u1", "me/messages", "GET", start, end, end.Sub(start), true, nil)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := mock.ExpectationsWereMet(); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("async logging did not satisfy expectations: %v", err)
	}
}

func TestMailServiceProcessUserMailActivity_Error(t *testing.T) {
	svc, mock, cleanup := makeMailServiceForErrorPath(t)
	defer cleanup()

	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
		WithArgs("u1").
		WillReturnError(errors.New("lookup failed"))

	err := svc.ProcessUserMailActivity(context.Background(), "u1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to process mail activity")
}

func TestMailServiceProcessAllUserMailActivity_GetUsersError(t *testing.T) {
	svc, mock, cleanup := makeMailServiceForErrorPath(t)
	defer cleanup()

	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+order by user_id`).
		WillReturnError(sql.ErrNoRows)

	err := svc.ProcessAllUserMailActivity(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get user IDs")
}

func TestMailServiceProcessAllUserMailActivity_ProcessingOutcomes(t *testing.T) {
	svc, mock, cleanup := makeMailServiceForErrorPath(t)
	defer cleanup()

	originalProcess := processUserMailActivity
	t.Cleanup(func() {
		processUserMailActivity = originalProcess
	})

	rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
		AddRow(1, "u1", "a", "r", time.Now(), "Bearer").
		AddRow(2, "u2", "a", "r", time.Now(), "Bearer").
		AddRow(3, "u3", "a", "r", time.Now(), "Bearer")
	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+order by user_id`).
		WillReturnRows(rows)

	var called []string
	processUserMailActivity = func(s *MailService, ctx context.Context, userID string) error {
		called = append(called, userID)
		if userID == "u2" {
			return errors.New("boom")
		}
		return nil
	}

	err := svc.ProcessAllUserMailActivity(context.Background())
	require.Error(t, err)
	require.EqualError(t, err, "failed to process mail activity for 1 out of 3 users")
	sort.Strings(called)
	require.Equal(t, []string{"u1", "u2", "u3"}, called)
}

func TestMailServiceProcessAllUserMailActivity_AllSuccess(t *testing.T) {
	svc, mock, cleanup := makeMailServiceForErrorPath(t)
	defer cleanup()

	originalProcess := processUserMailActivity
	t.Cleanup(func() {
		processUserMailActivity = originalProcess
	})

	rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
		AddRow(1, "u1", "a", "r", time.Now(), "Bearer").
		AddRow(2, "u2", "a", "r", time.Now(), "Bearer")
	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+order by user_id`).
		WillReturnRows(rows)

	var called []string
	processUserMailActivity = func(s *MailService, ctx context.Context, userID string) error {
		called = append(called, userID)
		return nil
	}

	err := svc.ProcessAllUserMailActivity(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"u1", "u2"}, called)
}

func TestMailServiceGetUserMail(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &MailService{logger: logger}

	originalNewClient := newMailGraphClient
	originalGetMessages := getMailMessages
	originalLog := logGraphAPICallHook
	t.Cleanup(func() {
		newMailGraphClient = originalNewClient
		getMailMessages = originalGetMessages
		logGraphAPICallHook = originalLog
	})
	logGraphAPICallHook = func(s *MailService, ctx context.Context, userID, endpoint, method string, startTime, endTime time.Time, duration time.Duration, success bool, err error) {
	}

	t.Run("graph client creation failure is wrapped", func(t *testing.T) {
		newMailGraphClient = func(credential *DatabaseTokenCredential) (*msgraphsdkgo.GraphServiceClient, error) {
			return nil, errors.New("graph client boom")
		}

		resp, err := svc.GetUserMail(context.Background(), "u1")
		require.Nil(t, resp)
		require.EqualError(t, err, "failed to create Graph client: graph client boom")
	})

	t.Run("graph api failure is wrapped", func(t *testing.T) {
		newMailGraphClient = func(credential *DatabaseTokenCredential) (*msgraphsdkgo.GraphServiceClient, error) {
			return &msgraphsdkgo.GraphServiceClient{}, nil
		}
		getMailMessages = func(ctx context.Context, client *msgraphsdkgo.GraphServiceClient) (models.MessageCollectionResponseable, error) {
			return nil, errors.New("graph request boom")
		}

		resp, err := svc.GetUserMail(context.Background(), "u1")
		require.Nil(t, resp)
		require.EqualError(t, err, "failed to get messages from Graph API: graph request boom")
	})

	t.Run("success returns converted mail response", func(t *testing.T) {
		newMailGraphClient = func(credential *DatabaseTokenCredential) (*msgraphsdkgo.GraphServiceClient, error) {
			return &msgraphsdkgo.GraphServiceClient{}, nil
		}
		getMailMessages = func(ctx context.Context, client *msgraphsdkgo.GraphServiceClient) (models.MessageCollectionResponseable, error) {
			id := "m1"
			subject := "hello"
			isRead := true
			received := time.Date(2026, 3, 10, 1, 0, 0, 0, time.UTC)
			msg := models.NewMessage()
			msg.SetId(&id)
			msg.SetSubject(&subject)
			msg.SetIsRead(&isRead)
			msg.SetReceivedDateTime(&received)

			resp := models.NewMessageCollectionResponse()
			resp.SetValue([]models.Messageable{msg})
			return resp, nil
		}

		resp, err := svc.GetUserMail(context.Background(), "u1")
		require.NoError(t, err)
		require.Len(t, resp.Value, 1)
		require.Equal(t, "m1", resp.Value[0].ID)
		require.Equal(t, "hello", resp.Value[0].Subject)
	})
}
