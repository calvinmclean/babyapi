package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"
	"github.com/stretchr/testify/require"
)

func TestAPI(t *testing.T) {
	defer os.RemoveAll("storage.json")

	api := createAPI()

	babytest.RunTableTest(t, api.Events, []babytest.TestCase[*babyapi.AnyResource]{
		{
			Name: "ErrorCreatingEventWithoutPassword",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				Body:   `{"Name": "Party"}`,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusBadRequest,
				Body:   `{"status":"Invalid request.","error":"missing required 'password' field"}`,
				Error:  "error posting resource: unexpected response with text: Invalid request.",
			},
		},
		{
			Name: "CreateEvent",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				Body:   `{"Name": "Party", "Password": "secret"}`,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Party","Contact":"","Date":"","Location":"","Details":""}`,
			},
		},
		{
			Name: "GetEventForbidden",
			Test: babytest.RequestFuncTest[*babyapi.AnyResource](func(getResponse babytest.PreviousResponseGetter, address string) *http.Request {
				id := getResponse("CreateEvent").Data.GetID()
				address = fmt.Sprintf("%s/events/%s", address, id)

				r, err := http.NewRequest(http.MethodGet, address, http.NoBody)
				require.NoError(t, err)
				return r
			}),
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
			},
		},
		{
			Name: "GetEvent",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   http.MethodGet,
				RawQuery: "password=secret",
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateEvent").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Party","Contact":"","Date":"","Location":"","Details":""}`,
			},
		},
		{
			Name: "SearchEventsForbidden",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   babyapi.MethodSearch,
				RawQuery: "password=secret",
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateEvent").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error getting all resources: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "SearchEventsForbiddenUsingRequestFuncTest",
			Test: babytest.RequestFuncTest[*babyapi.AnyResource](func(getResponse babytest.PreviousResponseGetter, address string) *http.Request {
				r, err := http.NewRequest(http.MethodGet, address+"/events", http.NoBody)
				require.NoError(t, err)
				return r
			}),
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
			},
		},
		{
			Name: "GetEventWithInvalidInvite",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   http.MethodGet,
				RawQuery: "invite=DoesNotExist",
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateEvent").Data.GetID()
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error getting resource: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "PUTNotAllowed",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   http.MethodPut,
				RawQuery: "password=secret",
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateEvent").Data.GetID()
				},
				BodyFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return fmt.Sprintf(`{"id": "%s", "name": "New Name"}`, getResponse("CreateEvent").Data.GetID())
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusBadRequest,
				Body:   `{"status":"Invalid request.","error":"PUT not allowed"}`,
				Error:  "error putting resource: unexpected response with text: Invalid request.",
			},
		},
		{
			Name: "CannotCreateInviteWithoutEventPassword",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodPost,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				Body: `{"Name": "Name"}`,
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error posting resource: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "CreateInvite",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   http.MethodPost,
				RawQuery: "password=secret",
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				Body: `{"Name": "Firstname Lastname"}`,
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}`,
			},
		},
		{
			Name: "GetInvite",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateInvite").Data.GetID()
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}`,
			},
		},
		{
			Name: "ListInvites",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   babyapi.MethodSearch,
				RawQuery: "password=secret",
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateInvite").Data.GetID()
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"items":\[{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}]`,
			},
		},
		{
			Name: "ListInvitesForbidden",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: babyapi.MethodSearch,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateInvite").Data.GetID()
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error getting all resources: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "ListInviteUsingRequestFuncTest",
			Test: babytest.RequestFuncTest[*babyapi.AnyResource](func(getResponse babytest.PreviousResponseGetter, address string) *http.Request {
				id := getResponse("CreateEvent").Data.GetID()
				address = fmt.Sprintf("%s/events/%s/invites", address, id)

				r, err := http.NewRequest(babyapi.MethodSearch, address, http.NoBody)
				require.NoError(t, err)

				q := r.URL.Query()
				q.Add("password", "secret")
				r.URL.RawQuery = q.Encode()

				return r
			}),
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"items":\[{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}]`,
			},
		},
		{
			Name: "GetEventWithInviteIDAsPassword",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodGet,
				RawQueryFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return "invite=" + getResponse("CreateInvite").Data.GetID()
				},
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateInvite").Data.GetID()
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}`,
			},
		},
		{
			Name: "DeleteInvite",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method: http.MethodDelete,
				ParentIDsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{getResponse("CreateEvent").Data.GetID()}
				},
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateInvite").Data.GetID()
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusOK,
				NoBody: true,
			},
		},
		{
			Name: "PatchErrorNotConfigured",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   http.MethodPatch,
				RawQuery: "password=secret",
				IDFunc: func(getResponse babytest.PreviousResponseGetter) string {
					return getResponse("CreateEvent").Data.GetID()
				},
				Body: `{"Name": "NEW"}`,
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusMethodNotAllowed,
				Error:  "error patching resource: unexpected response with text: Method not allowed.",
				Body:   `{"status":"Method not allowed."}`,
			},
		},
	})
}

func TestIndividualTest(t *testing.T) {
	defer os.RemoveAll("storage.json")

	api := createAPI()

	client, stop := babytest.NewTestClient[*Event](t, api.Events)
	defer stop()

	babytest.TestCase[*Event]{
		Name: "CreateEvent",
		Test: babytest.RequestTest[*Event]{
			Method: http.MethodPost,
			Body:   `{"Name": "Party", "Password": "secret"}`,
		},
		ExpectedResponse: babytest.ExpectedResponse{
			Status:     http.StatusCreated,
			BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Party","Contact":"","Date":"","Location":"","Details":""}`,
		},
		Assert: func(r *babytest.Response[*Event]) {
			require.Equal(t, "Party", r.Data.Name)
		},
	}.Run(t, client)

	resp := babytest.TestCase[*Event]{
		Name: "CreateEvent",
		Test: babytest.RequestTest[*Event]{
			Method: http.MethodPost,
			Body:   `{"Name": "Party", "Password": "secret"}`,
		},
		ExpectedResponse: babytest.ExpectedResponse{
			Status:     http.StatusCreated,
			BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Party","Contact":"","Date":"","Location":"","Details":""}`,
		},
		Assert: func(r *babytest.Response[*Event]) {
			require.Equal(t, "Party", r.Data.Name)
		},
	}.RunWithResponse(t, client)
	require.NotNil(t, resp)
}
