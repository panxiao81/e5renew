package services

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/stretchr/testify/require"

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
