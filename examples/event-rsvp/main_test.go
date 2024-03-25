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
			Name: "GetAllEventsForbidden",
			Test: babytest.RequestTest[*babyapi.AnyResource]{
				Method:   babytest.MethodGetAll,
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
			Name: "GetAllEventsForbiddenUsingRequestFuncTest",
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
				Method:   babytest.MethodGetAll,
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
			Name: "ListInviteUsingRequestFuncTest",
			Test: babytest.RequestFuncTest[*babyapi.AnyResource](func(getResponse babytest.PreviousResponseGetter, address string) *http.Request {
				id := getResponse("CreateEvent").Data.GetID()
				address = fmt.Sprintf("%s/events/%s/invites", address, id)

				r, err := http.NewRequest(babytest.MethodGetAll, address, http.NoBody)
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

func TestCLI(t *testing.T) {
	defer os.RemoveAll("storage.json")

	api := createAPI()

	babytest.RunTableTest(t, api.Events, []babytest.TestCase[*babyapi.AnyResource]{
		{
			Name: "ErrorCreatingEventWithoutPassword",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				Args:    []string{"event", "post", "-d", `{"Name": "Party"}`},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusBadRequest,
				Body:   `{"status":"Invalid request.","error":"missing required 'password' field"}`,
				Error:  "error running client from CLI: error running Post: error posting resource: unexpected response with text: Invalid request.",
			},
		},
		{
			Name: "CreateEvent",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				Args:    []string{"event", "post", "-d", `{"Name": "Party", "Password": "secret"}`},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:       http.StatusCreated,
				BodyRegexp:   `{"id":"[0-9a-v]{20}","Name":"Party","Contact":"","Date":"","Location":"","Details":""}`,
				CLIOutRegexp: `{\s+"Contact": "",\s+"Date": "",\s+"Details": "",\s+"Location": "",\s+"Name": "Party",\s+"id": "[0-9a-v]{20}"\s+}\s*`,
			},
		},
		{
			Name: "GetEventForbidden",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{"event", "get", getResponse("CreateEvent").Data.GetID()}
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error running client from CLI: error running Get: error getting resource: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "GetEvent",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{"event", "get", getResponse("CreateEvent").Data.GetID(), "-q", "password=secret"}
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Party","Contact":"","Date":"","Location":"","Details":""}`,
			},
		},
		{
			Name: "GetEventWithInvalidInvite",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{"event", "get", getResponse("CreateEvent").Data.GetID(), "-q", "invite=DoesNotExist"}
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error running client from CLI: error running Get: error getting resource: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "PUTNotAllowed",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					return []string{
						"event", "put",
						getResponse("CreateEvent").Data.GetID(),
						"-d", fmt.Sprintf(`{"id": "%s", "name": "New Name"}`, getResponse("CreateEvent").Data.GetID()),
						"-q", "password=secret",
					}
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusBadRequest,
				Body:   `{"status":"Invalid request.","error":"PUT not allowed"}`,
				Error:  "error running client from CLI: error running Put: error putting resource: unexpected response with text: Invalid request.",
			},
		},
		{
			Name: "CannotCreateInviteWithoutEventPassword",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{"invite", "post", eventID, "-d", `{"Name": "Name"}`}
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusForbidden,
				Body:   `{"status":"Forbidden"}`,
				Error:  "error running client from CLI: error running Post: error posting resource: unexpected response with text: Forbidden",
			},
		},
		{
			Name: "CreateInvite",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{"invite", "post", eventID, "-d", `{"Name": "Firstname Lastname"}`, "-q", "password=secret"}
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusCreated,
				BodyRegexp: `{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}`,
			},
		},
		{
			Name: "GetInvite",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{
						"invite", "get",
						getResponse("CreateInvite").Data.GetID(), eventID,
						"-q", "password=secret",
					}
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
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{
						"invite", "list", eventID, "-q", "password=secret",
					}
				},
			},
			ClientName: "Invite",
			ExpectedResponse: babytest.ExpectedResponse{
				Status:     http.StatusOK,
				BodyRegexp: `{"items":\[{"id":"[0-9a-v]{20}","Name":"Firstname Lastname","Contact":"","EventID":"[0-9a-v]{20}","RSVP":null}]`,
			},
		},
		{
			Name: "GetEventWithInviteIDAsPassword",
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{
						"invite", "get",
						getResponse("CreateInvite").Data.GetID(), eventID,
						"-q", "invite=" + getResponse("CreateInvite").Data.GetID(),
					}
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
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{
						"invite", "delete",
						getResponse("CreateInvite").Data.GetID(), eventID,
					}
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
			Test: babytest.CommandLineTest[*babyapi.AnyResource]{
				Command: api.Events.Command,
				ArgsFunc: func(getResponse babytest.PreviousResponseGetter) []string {
					eventID := getResponse("CreateEvent").Data.GetID()
					return []string{"event", "patch", eventID, "-d", `{"Name": "NEW"}`, "-q", "password=secret"}
				},
			},
			ExpectedResponse: babytest.ExpectedResponse{
				Status: http.StatusMethodNotAllowed,
				Body:   `{"status":"Method not allowed."}`,
				Error:  "error running client from CLI: error running Patch: error patching resource: unexpected response with text: Method not allowed.",
			},
		},
	})
}
