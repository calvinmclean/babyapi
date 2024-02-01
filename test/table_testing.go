package babytest

import (
	"testing"

	"github.com/calvinmclean/babyapi"
)

// PreviousResponseGetter is used to get the output of previous tests in a TableTest
type PreviousResponseGetter func(testName string) *Response[*babyapi.AnyResource]

// RunTableTest will start the provided API and execute all provided tests in-order. This allows the usage of a
// PreviousResponseGetter in each test to access data from previous tests. The API's ClientMap is used to execute
// tests with child clients if the test uses ClientName field
func RunTableTest[T babyapi.Resource](t *testing.T, api *babyapi.API[T], tests []TestCase[*babyapi.AnyResource]) {
	client, stop := NewTestAnyClient[T](t, api)
	defer stop()

	results := map[string]*Response[*babyapi.AnyResource]{}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			testClient := client

			if tt.ClientName != "" {
				clientMap := api.CreateClientMap(client)
				var ok bool
				testClient, ok = clientMap[tt.ClientName]
				if !ok {
					t.Errorf("missing subclient for key %q. available clients are %v", tt.ClientName, clientMap)
				}
			}

			results[tt.Name] = tt.run(t, testClient, func(testName string) *Response[*babyapi.AnyResource] {
				return results[testName]
			})
		})
	}
}
