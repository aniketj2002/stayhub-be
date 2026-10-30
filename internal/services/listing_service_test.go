package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/domain/models"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func int64Ptr(i int64) *int64 { return &i }

func TestEvaluateListingChecks(t *testing.T) {
	hostID := uuid.New()
	authorizedUploaders := map[uuid.UUID]bool{hostID: true}
	validPolicies := map[string]bool{
		"flexible": true,
		"moderate": true,
		"firm":     true,
		"strict":   true,
	}

	var mediaIDs []uuid.UUID
	var mediaRecords []models.Media
	for i := 0; i < 5; i++ {
		id := uuid.New()
		mediaIDs = append(mediaIDs, id)
		mediaRecords = append(mediaRecords, models.Media{
			ID:         id,
			Status:     "uploaded",
			UploadedBy: hostID,
		})
	}

	validVersion := models.ListingVersion{
		ID:                 uuid.New(),
		ListingID:          uuid.New(),
		VersionNo:          1,
		PropertyType:       strPtr("apartment"),
		Title:              strPtr("Alfama Riverfront Loft"),
		Description:        strPtr("Sunlit apartment overlooking the Tagus."),
		AddressLine1:       strPtr("Rua dos Remedios 42"),
		City:               strPtr("Lisbon"),
		Country:            strPtr("PT"),
		Timezone:           "Europe/Lisbon",
		MaxGuests:          intPtr(4),
		BasePriceMinor:     int64Ptr(14500),
		CleaningFeeMinor:   3000,
		Currency:           "EUR",
		CheckInTime:        "15:00",
		CheckOutTime:       "11:00",
		HouseRules:         strPtr("No smoking. Quiet hours after 22:00."),
		CancellationPolicy: strPtr("moderate"),
		MediaIDs:           mediaIDs,
	}

	t.Run("passes all 3 checks when complete and >= 5 uploaded photos", func(t *testing.T) {
		results, allPassed := EvaluateListingChecks(validVersion, validPolicies, mediaRecords, authorizedUploaders)
		if !allPassed {
			t.Fatalf("expected allPassed=true, got false: %+v", results)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 check results, got %d", len(results))
		}
		for _, r := range results {
			if r.Outcome != "pass" {
				t.Fatalf("expected check %s to pass, got %s (%v)", r.CheckCode, r.Outcome, r.Detail)
			}
		}
	})

	t.Run("fails min_photos when fewer than 5 photos are attached", func(t *testing.T) {
		v := validVersion
		v.MediaIDs = mediaIDs[:4]
		results, allPassed := EvaluateListingChecks(v, validPolicies, mediaRecords[:4], authorizedUploaders)
		if allPassed {
			t.Fatalf("expected allPassed=false when only 4 photos provided")
		}
		var minPhotosOutcome string
		for _, r := range results {
			if r.CheckCode == "min_photos" {
				minPhotosOutcome = r.Outcome
			}
		}
		if minPhotosOutcome != "fail" {
			t.Fatalf("expected min_photos outcome 'fail', got %q", minPhotosOutcome)
		}
	})

	t.Run("fails required_fields and house_rules_policy when missing", func(t *testing.T) {
		v := validVersion
		v.Title = nil
		v.HouseRules = strPtr("   ")
		results, allPassed := EvaluateListingChecks(v, validPolicies, mediaRecords, authorizedUploaders)
		if allPassed {
			t.Fatalf("expected allPassed=false")
		}
		outcomes := map[string]string{}
		for _, r := range results {
			outcomes[r.CheckCode] = r.Outcome
		}
		if outcomes["required_fields"] != "fail" || outcomes["house_rules_policy"] != "fail" {
			t.Fatalf("expected required_fields and house_rules_policy to fail, got %+v", outcomes)
		}
	})
}
