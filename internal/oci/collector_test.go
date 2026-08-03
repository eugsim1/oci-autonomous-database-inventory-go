package oci

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/model"
	"github.com/oracle/oci-go-sdk/v65/common"
)

type fakeRichServiceError struct{}

func (fakeRichServiceError) Error() string {
	return "Authorization failed or requested resource not found."
}
func (fakeRichServiceError) GetHTTPStatusCode() int { return 404 }
func (fakeRichServiceError) GetMessage() string {
	return "Authorization failed or requested resource not found."
}
func (fakeRichServiceError) GetCode() string          { return "NotAuthorizedOrNotFound" }
func (fakeRichServiceError) GetOpcRequestID() string  { return "opc-request-id" }
func (fakeRichServiceError) GetTargetService() string { return "Database" }
func (fakeRichServiceError) GetOperationName() string { return "GetAutonomousDatabase" }
func (fakeRichServiceError) GetTimestamp() common.SDKTime {
	return common.SDKTime{Time: time.Date(2026, 8, 3, 13, 56, 9, 0, time.UTC)}
}
func (fakeRichServiceError) GetRequestTarget() string {
	return "GET https://database.eu-frankfurt-1.oraclecloud.com/20160918/autonomousDatabases/example"
}
func (fakeRichServiceError) GetClientVersion() string { return "Oracle-GoSDK/65.117.1" }
func (fakeRichServiceError) GetOperationReferenceLink() string {
	return "https://example.test/operation"
}
func (fakeRichServiceError) GetErrorTroubleshootingLink() string {
	return "https://example.test/troubleshooting"
}

func TestSelectRegionsUsesOnlyReadySubscriptions(t *testing.T) {
	regions := []model.Region{
		{Name: "us-ashburn-1", Status: "READY"},
		{Name: "eu-paris-1", Status: "IN_PROGRESS"},
		{Name: "uk-london-1", Status: "READY"},
	}
	got, err := selectRegions(regions, nil)
	if err != nil {
		t.Fatalf("selectRegions() error = %v", err)
	}
	want := []string{"uk-london-1", "us-ashburn-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectRegions() = %#v, want %#v", got, want)
	}
}

func TestSelectRegionsRejectsNonReadyRequest(t *testing.T) {
	regions := []model.Region{{Name: "eu-paris-1", Status: "IN_PROGRESS"}}
	if _, err := selectRegions(regions, []string{"eu-paris-1"}); err == nil {
		t.Fatal("selectRegions() expected an error")
	}
}

func TestCollectionErrorCapturesRichOCIDiagnostics(t *testing.T) {
	got := collectionErrorForRef("get_autonomous_database", resourceRef{
		Region:         "eu-frankfurt-1",
		ID:             "ocid1.autonomousdatabase.oc1.eu-frankfurt-1.example",
		CompartmentID:  "ocid1.compartment.oc1..example",
		DisplayName:    "finance",
		LifecycleState: "AVAILABLE",
		TimeCreated:    "2026-07-01T10:00:00Z",
	}, fakeRichServiceError{})

	if got.HTTPStatusCode != 404 || got.ServiceCode != "NotAuthorizedOrNotFound" {
		t.Fatalf("service metadata = HTTP %d %q", got.HTTPStatusCode, got.ServiceCode)
	}
	if got.Retryable == nil || *got.Retryable {
		t.Fatalf("Retryable = %v, want false", got.Retryable)
	}
	if got.OperationName != "GetAutonomousDatabase" || got.OPCRequestID != "opc-request-id" {
		t.Fatalf("rich request metadata was not preserved: %#v", got)
	}
	if got.SearchDisplayName != "finance" || got.SearchLifecycleState != "AVAILABLE" {
		t.Fatalf("Search metadata was not preserved: %#v", got)
	}
	if !strings.Contains(got.Diagnosis, "AUTONOMOUS_DATABASE_INSPECT") || len(got.SuggestedActions) == 0 {
		t.Fatalf("diagnosis is incomplete: %#v", got)
	}
}
