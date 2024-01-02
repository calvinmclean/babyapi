package main

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/calvinmclean/babyapi"
	babyapi_testing "github.com/calvinmclean/babyapi/testing"
	"github.com/stretchr/testify/require"
)

func TestAPI(t *testing.T) {
	api := createAPI()
	serverURL, stop := babyapi_testing.TestServe[*Event](t, api)
	defer stop()
	eventClient := api.Client(serverURL)
	inviteClient := babyapi.NewSubClient[*Event, *Invite](eventClient, "/invites")

	defer os.RemoveAll("storage.json")

	t.Run("ErrorCreatingEventWithoutPassword", func(t *testing.T) {
		_, err := eventClient.Post(context.Background(), &Event{Name: "Party"})
		require.Error(t, err)
		require.Equal(t, "error posting resource: unexpected response with text: Invalid request.", err.Error())
	})

	var event *Event
	t.Run("CreateEvent", func(t *testing.T) {
		resp, err := eventClient.Post(context.Background(), &Event{Name: "Party", Password: "secret"})
		require.NoError(t, err)

		event = resp.Data
		require.Empty(t, event.Salt)
		require.Empty(t, event.Key)
	})

	t.Run("GetAllEventsError", func(t *testing.T) {
		_, err := eventClient.GetAll(context.Background(), nil)
		require.Error(t, err)
		require.Equal(t, "error getting all resources: unexpected response with text: Forbidden", err.Error())
	})

	t.Run("GetEventWithoutPassword", func(t *testing.T) {
		_, err := eventClient.Get(context.Background(), event.GetID())
		require.Error(t, err)
		require.Equal(t, "error getting resource: unexpected response with text: Forbidden", err.Error())
	})

	t.Run("GetEventWithPassword", func(t *testing.T) {
		eventClient.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = "password=secret"
			return nil
		})
		defer eventClient.SetRequestEditor(babyapi.DefaultRequestEditor)

		_, err := eventClient.Get(context.Background(), event.GetID())
		require.NoError(t, err)
	})

	t.Run("GetEventWithInvalidInvite", func(t *testing.T) {
		eventClient.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = "&invite=DoesNotExist"
			return nil
		})
		defer eventClient.SetRequestEditor(babyapi.DefaultRequestEditor)

		_, err := eventClient.Get(context.Background(), event.GetID())
		require.Error(t, err)
		require.Equal(t, "error getting resource: unexpected response with text: Forbidden", err.Error())
	})

	t.Run("PUTNotAllowed", func(t *testing.T) {
		eventClient.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = "password=secret"
			return nil
		})
		defer eventClient.SetRequestEditor(babyapi.DefaultRequestEditor)

		_, err := eventClient.Put(context.Background(), &Event{
			DefaultResource: event.DefaultResource,
			Name:            "New Name",
		})
		require.Error(t, err)
		require.Equal(t, "error putting resource: unexpected response with text: Invalid request.", err.Error())
	})

	t.Run("CannotCreateInviteWithoutEventPassword", func(t *testing.T) {
		_, err := inviteClient.Post(context.Background(), &Invite{Name: "Name"}, event.GetID())
		require.Error(t, err)
		require.Equal(t, "error posting resource: unexpected response with text: Forbidden", err.Error())
	})

	var invite *Invite
	t.Run("CreateInvite", func(t *testing.T) {
		inviteClient.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = "password=secret"
			return nil
		})
		defer inviteClient.SetRequestEditor(babyapi.DefaultRequestEditor)

		resp, err := inviteClient.Post(context.Background(), &Invite{Name: "Firstname Lastname"}, event.GetID())
		require.NoError(t, err)

		invite = resp.Data
		require.Equal(t, event.GetID(), invite.EventID)
	})

	t.Run("GetInvite", func(t *testing.T) {
		resp, err := inviteClient.Get(context.Background(), invite.GetID(), event.GetID())
		require.NoError(t, err)

		invite = resp.Data
		require.Equal(t, event.GetID(), invite.EventID)
	})

	t.Run("GetEventWithInviteIDAsPassword", func(t *testing.T) {
		eventClient.SetRequestEditor(func(r *http.Request) error {
			r.URL.RawQuery = "&invite=" + invite.GetID()
			return nil
		})
		defer eventClient.SetRequestEditor(babyapi.DefaultRequestEditor)

		_, err := eventClient.Get(context.Background(), event.GetID())
		require.NoError(t, err)
	})
}
