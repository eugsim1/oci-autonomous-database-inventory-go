package oci

import (
	"reflect"
	"testing"

	"github.com/eugsim1/oci-autonomous-database-inventory-go/internal/model"
)

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
