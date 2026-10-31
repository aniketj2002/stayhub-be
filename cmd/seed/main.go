package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/domain/enums"
	"github.com/stayhub/backend/internal/domain/models"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

type seedListingSpec struct {
	ListingID          uuid.UUID
	VersionID          uuid.UUID
	HostID             uuid.UUID
	CohostID           *uuid.UUID
	PropertyType       string
	Title              string
	Description        string
	AddressLine1       string
	City               string
	Region             string
	Country            string
	PostalCode         string
	Timezone           string
	MaxGuests          int
	BasePriceMinor     int64
	CleaningFeeMinor   int64
	Currency           string
	InstantBook        bool
	CheckInTime        string
	CheckOutTime       string
	HouseRules         string
	CancellationPolicy string
	Amenities          []string
	PhotoURLs          []string
}

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	database, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer database.Close()

	pwdHash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash seed password: %v", err)
	}

	stayhubHash, err := bcrypt.GenerateFromPassword([]byte("stayhub"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash stayhub admin password: %v", err)
	}

	// Deterministic UUIDs so `make seed` is idempotent across runs
	adminID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	adminComID := uuid.MustParse("00000000-0000-4000-8000-000000000099")
	hostInesID := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	hostKenjiID := uuid.MustParse("10000000-0000-4000-8000-000000000002")
	guestClaireID := uuid.MustParse("20000000-0000-4000-8000-000000000001")
	guestArjunID := uuid.MustParse("20000000-0000-4000-8000-000000000002")

	now := time.Now().UTC()

	err = database.WithinTx(ctx, func(txCtx context.Context) error {
		conn := database.Conn(txCtx)

		// 1. Seed Admin Users
		adminSeeds := []struct {
			id       uuid.UUID
			email    string
			hash     string
			fullName string
			role     string
		}{
			{adminComID, "admin@stayhub.com", string(stayhubHash), "StayHub Super Admin", "super_admin"},
			{adminID, "admin@stayhub.dev", string(pwdHash), "StayHub Platform Admin", "super_admin"},
		}

		for _, a := range adminSeeds {
			_, err := conn.Exec(txCtx, `
				INSERT INTO admin_users (id, email, password_hash, full_name, role)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (email) DO UPDATE
				SET password_hash = EXCLUDED.password_hash,
				    full_name = EXCLUDED.full_name,
				    role = EXCLUDED.role,
				    updated_at = now()
			`, a.id, a.email, a.hash, a.fullName, a.role)
			if err != nil {
				return fmt.Errorf("seed admin_users (%s): %w", a.email, err)
			}
		}

		// 2. Seed Users (Hosts & Guests)
		users := []struct {
			ID       uuid.UUID
			Email    string
			FullName string
			IsGuest  bool
			IsHost   bool
		}{
			{hostInesID, "ines.costa@stayhub.dev", "Inês Costa", true, true},
			{hostKenjiID, "kenji.takahashi@stayhub.dev", "Kenji Takahashi", true, true},
			{guestClaireID, "claire.dubois@stayhub.dev", "Claire Dubois", true, false},
			{guestArjunID, "arjun.mehta@stayhub.dev", "Arjun Mehta", true, false},
		}

		for i := range users {
			u := &users[i]
			err := conn.QueryRow(txCtx, `
				INSERT INTO users (id, email, password_hash, full_name, is_guest, is_host)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (email) DO UPDATE
				SET password_hash = EXCLUDED.password_hash,
				    full_name = EXCLUDED.full_name,
				    is_guest = EXCLUDED.is_guest,
				    is_host = EXCLUDED.is_host,
				    updated_at = now()
				RETURNING id
			`, u.ID, u.Email, string(pwdHash), u.FullName, u.IsGuest, u.IsHost).Scan(&u.ID)
			if err != nil {
				return fmt.Errorf("seed user %s: %w", u.Email, err)
			}
		}
		hostInesID = users[0].ID
		hostKenjiID = users[1].ID
		guestClaireID = users[2].ID
		guestArjunID = users[3].ID

		// 3. Seed 6 Live Listings across Lisbon, Goa, and Kyoto
		listingSpecs := []seedListingSpec{
			{
				ListingID:          uuid.MustParse("30000000-0000-4000-8000-000000000001"),
				VersionID:          uuid.MustParse("31000000-0000-4000-8000-000000000001"),
				HostID:             hostInesID,
				CohostID:           &hostKenjiID,
				PropertyType:       "apartment",
				Title:              "Alfama Terracotta Loft with Tagus River Terrace",
				Description:        "Sun-drenched top-floor loft in historic Alfama featuring hand-painted azulejos, French oak floors, a dedicated workspace, and a private terrace overlooking the Tagus River.",
				AddressLine1:       "Rua dos Remédios 48",
				City:               "Lisbon",
				Region:             "Lisboa",
				Country:            "PT",
				PostalCode:         "1100-404",
				Timezone:           "Europe/Lisbon",
				MaxGuests:          4,
				BasePriceMinor:     14500, // €145.00
				CleaningFeeMinor:   3500,  // €35.00
				Currency:           "EUR",
				InstantBook:        true,
				CheckInTime:        "15:00",
				CheckOutTime:       "11:00",
				HouseRules:         "No smoking indoors. Quiet hours after 22:00 to respect historic neighborhood residents. No unregistered events.",
				CancellationPolicy: "moderate",
				Amenities:          []string{"wifi", "kitchen", "air_conditioning", "dedicated_workspace", "washer", "waterfront", "self_check_in", "smoke_alarm"},
				PhotoURLs: []string{
					"https://images.unsplash.com/photo-1502672260266-1c1ef2d93688?auto=format&fit=crop&w=1200&q=80#lisbon-alfama-1",
					"https://images.unsplash.com/photo-1522708323590-d24dbb6b0267?auto=format&fit=crop&w=1200&q=80#lisbon-alfama-2",
					"https://images.unsplash.com/photo-1493809842364-78817add7ffb?auto=format&fit=crop&w=1200&q=80#lisbon-alfama-3",
					"https://images.unsplash.com/photo-1560448204-e02f11c3d0e2?auto=format&fit=crop&w=1200&q=80#lisbon-alfama-4",
					"https://images.unsplash.com/photo-1484154218962-a197022b5858?auto=format&fit=crop&w=1200&q=80#lisbon-alfama-5",
				},
			},
			{
				ListingID:          uuid.MustParse("30000000-0000-4000-8000-000000000002"),
				VersionID:          uuid.MustParse("31000000-0000-4000-8000-000000000002"),
				HostID:             hostInesID,
				PropertyType:       "house",
				Title:              "Príncipe Real Townhouse & Citrus Courtyard",
				Description:        "Restored 19th-century Pombaline townhouse steps from Jardim do Príncipe Real. Features a private lemon-tree courtyard, chef's kitchen, and three airy bedrooms.",
				AddressLine1:       "Rua Dom Pedro V 82",
				City:               "Lisbon",
				Region:             "Lisboa",
				Country:            "PT",
				PostalCode:         "1250-094",
				Timezone:           "Europe/Lisbon",
				MaxGuests:          6,
				BasePriceMinor:     21000, // €210.00
				CleaningFeeMinor:   4500,  // €45.00
				Currency:           "EUR",
				InstantBook:        false, // Request-to-Book
				CheckInTime:        "15:00",
				CheckOutTime:       "11:00",
				HouseRules:         "Please remove outdoor shoes on the upper bedroom level. No commercial photography without prior host permission.",
				CancellationPolicy: "flexible",
				Amenities:          []string{"wifi", "kitchen", "washer", "dryer", "air_conditioning", "heating", "indoor_fireplace", "smoke_alarm", "first_aid_kit"},
				PhotoURLs: []string{
					"https://images.unsplash.com/photo-1600585154340-be6161a56a0c?auto=format&fit=crop&w=1200&q=80#lisbon-principe-1",
					"https://images.unsplash.com/photo-1600596542815-ffad4c1539a9?auto=format&fit=crop&w=1200&q=80#lisbon-principe-2",
					"https://images.unsplash.com/photo-1600607687939-ce8a6c25118c?auto=format&fit=crop&w=1200&q=80#lisbon-principe-3",
					"https://images.unsplash.com/photo-1600566753376-12c8ab7fb75b?auto=format&fit=crop&w=1200&q=80#lisbon-principe-4",
					"https://images.unsplash.com/photo-1600585154526-990dced4db0d?auto=format&fit=crop&w=1200&q=80#lisbon-principe-5",
				},
			},
			{
				ListingID:          uuid.MustParse("30000000-0000-4000-8000-000000000003"),
				VersionID:          uuid.MustParse("31000000-0000-4000-8000-000000000003"),
				HostID:             hostInesID,
				PropertyType:       "villa",
				Title:              "Assagao Portuguese Villa with Private Plunge Pool",
				Description:        "Heritage Indo-Portuguese laterite villa nestled in lush Assagao palm groves. Includes a private salt-water plunge pool, wraparound balcão veranda, and high-speed fiber Wi-Fi.",
				AddressLine1:       "Bairo Alto, Assagao",
				City:               "Goa",
				Region:             "North Goa",
				Country:            "IN",
				PostalCode:         "403507",
				Timezone:           "Asia/Kolkata",
				MaxGuests:          6,
				BasePriceMinor:     18000, // €180.00
				CleaningFeeMinor:   4000,  // €40.00
				Currency:           "EUR",
				InstantBook:        true,
				CheckInTime:        "14:00",
				CheckOutTime:       "11:00",
				HouseRules:         "Pool hours are 07:00 to 22:00. Please conserve water and power AC units off when leaving the villa.",
				CancellationPolicy: "firm",
				Amenities:          []string{"wifi", "kitchen", "pool", "air_conditioning", "free_parking", "dedicated_workspace", "breakfast", "smoke_alarm", "fire_extinguisher"},
				PhotoURLs: []string{
					"https://images.unsplash.com/photo-1580587771525-78b9dba3b914?auto=format&fit=crop&w=1200&q=80#goa-assagao-1",
					"https://images.unsplash.com/photo-1613977257363-707ba9348227?auto=format&fit=crop&w=1200&q=80#goa-assagao-2",
					"https://images.unsplash.com/photo-1512917774080-9991f1c4c750?auto=format&fit=crop&w=1200&q=80#goa-assagao-3",
					"https://images.unsplash.com/photo-1613490493576-7fde63acd811?auto=format&fit=crop&w=1200&q=80#goa-assagao-4",
					"https://images.unsplash.com/photo-1540555700478-4be289fbecef?auto=format&fit=crop&w=1200&q=80#goa-assagao-5",
				},
			},
			{
				ListingID:          uuid.MustParse("30000000-0000-4000-8000-000000000004"),
				VersionID:          uuid.MustParse("31000000-0000-4000-8000-000000000004"),
				HostID:             hostInesID,
				PropertyType:       "bungalow",
				Title:              "Morjim Beachfront Palm Bungalow",
				Description:        "Tranquil timber-and-lime bungalow just 60 meters from Morjim turtle beach. Wake up to Arabian Sea breezes, outdoor rain showers, and sunset views from your private hammock deck.",
				AddressLine1:       "Vithaldas Waddo, Morjim",
				City:               "Goa",
				Region:             "North Goa",
				Country:            "IN",
				PostalCode:         "403512",
				Timezone:           "Asia/Kolkata",
				MaxGuests:          3,
				BasePriceMinor:     11500, // €115.00
				CleaningFeeMinor:   2500,  // €25.00
				Currency:           "EUR",
				InstantBook:        false,
				CheckInTime:        "14:00",
				CheckOutTime:       "11:00",
				HouseRules:         "Morjim is an olive ridley turtle nesting zone: no bright beachfront floodlights or loud amplified music after 21:00.",
				CancellationPolicy: "moderate",
				Amenities:          []string{"wifi", "kitchen", "air_conditioning", "beachfront", "waterfront", "free_parking", "pets_allowed", "first_aid_kit"},
				PhotoURLs: []string{
					"https://images.unsplash.com/photo-1499793983690-e29da59ef1c2?auto=format&fit=crop&w=1200&q=80#goa-morjim-1",
					"https://images.unsplash.com/photo-1520250497591-112f2f40a3f4?auto=format&fit=crop&w=1200&q=80#goa-morjim-2",
					"https://images.unsplash.com/photo-1507525428034-b723cf961d3e?auto=format&fit=crop&w=1200&q=80#goa-morjim-3",
					"https://images.unsplash.com/photo-1540518614846-7ede433c4576?auto=format&fit=crop&w=1200&q=80#goa-morjim-4",
					"https://images.unsplash.com/photo-1510798831971-661eb04b3739?auto=format&fit=crop&w=1200&q=80#goa-morjim-5",
				},
			},
			{
				ListingID:          uuid.MustParse("30000000-0000-4000-8000-000000000005"),
				VersionID:          uuid.MustParse("31000000-0000-4000-8000-000000000005"),
				HostID:             hostKenjiID,
				PropertyType:       "house",
				Title:              "Higashiyama Cedar Machiya with Moss Garden",
				Description:        "Authentic 100-year-old Kyoto machiya meticulously restored by local carpenters. Features hinoki cypress soaking tub, tatami tea room viewing a private tsubo-niwa moss garden, and radiant floor heating.",
				AddressLine1:       "Gionmachi Minamigawa 570",
				City:               "Kyoto",
				Region:             "Kyoto",
				Country:            "JP",
				PostalCode:         "605-0074",
				Timezone:           "Asia/Tokyo",
				MaxGuests:          4,
				BasePriceMinor:     24000, // €240.00
				CleaningFeeMinor:   5000,  // €50.00
				Currency:           "EUR",
				InstantBook:        false, // Request-to-Book
				CheckInTime:        "15:00",
				CheckOutTime:       "10:00",
				HouseRules:         "Strictly no shoes beyond the genkan entryway. No wheeled luggage on tatami mats. Quiet residential lane at all hours.",
				CancellationPolicy: "strict",
				Amenities:          []string{"wifi", "kitchen", "washer", "air_conditioning", "heating", "hot_tub", "dedicated_workspace", "smoke_alarm", "carbon_monoxide_alarm"},
				PhotoURLs: []string{
					"https://images.unsplash.com/photo-1503899036084-c55cdd92da26?auto=format&fit=crop&w=1200&q=80#kyoto-machiya-1",
					"https://images.unsplash.com/photo-1493976040374-85c8e12f0c0e?auto=format&fit=crop&w=1200&q=80#kyoto-machiya-2",
					"https://images.unsplash.com/photo-1545569341-9eb8b30979d9?auto=format&fit=crop&w=1200&q=80#kyoto-machiya-3",
					"https://images.unsplash.com/photo-1528164344705-475426879c0d?auto=format&fit=crop&w=1200&q=80#kyoto-machiya-4",
					"https://images.unsplash.com/photo-1536098561742-ca998e48cbcc?auto=format&fit=crop&w=1200&q=80#kyoto-machiya-5",
				},
			},
			{
				ListingID:          uuid.MustParse("30000000-0000-4000-8000-000000000006"),
				VersionID:          uuid.MustParse("31000000-0000-4000-8000-000000000006"),
				HostID:             hostKenjiID,
				PropertyType:       "studio",
				Title:              "Kamogawa Riverfront Minimalist Studio",
				Description:        "Architect-designed studio apartment overlooking the Kamo River and Higashiyama mountains. Pour-over coffee bar, projector cinema, and effortless self check-in near Sanjo Station.",
				AddressLine1:       "Kiyamachi-dori Sanjo-sagaru 18",
				City:               "Kyoto",
				Region:             "Kyoto",
				Country:            "JP",
				PostalCode:         "604-8002",
				Timezone:           "Asia/Tokyo",
				MaxGuests:          2,
				BasePriceMinor:     13000, // €130.00
				CleaningFeeMinor:   3000,  // €30.00
				Currency:           "EUR",
				InstantBook:        true,
				CheckInTime:        "15:00",
				CheckOutTime:       "11:00",
				HouseRules:         "No smoking on the river balcony. Please sort recyclables into the labeled kitchen bins.",
				CancellationPolicy: "flexible",
				Amenities:          []string{"wifi", "kitchen", "air_conditioning", "heating", "dedicated_workspace", "waterfront", "self_check_in", "smoke_alarm"},
				PhotoURLs: []string{
					"https://images.unsplash.com/photo-1554995207-c18c203602cb?auto=format&fit=crop&w=1200&q=80#kyoto-kamo-1",
					"https://images.unsplash.com/photo-1513694203232-719a280e022f?auto=format&fit=crop&w=1200&q=80#kyoto-kamo-2",
					"https://images.unsplash.com/photo-1505691938895-1758d7feb511?auto=format&fit=crop&w=1200&q=80#kyoto-kamo-3",
					"https://images.unsplash.com/photo-1536376072261-38c75010e6c9?auto=format&fit=crop&w=1200&q=80#kyoto-kamo-4",
					"https://images.unsplash.com/photo-1502005229762-cf1b2da7c5d6?auto=format&fit=crop&w=1200&q=80#kyoto-kamo-5",
				},
			},
		}

		for idx, spec := range listingSpecs {
			// Insert 5 uploaded media items for this listing
			var mediaIDs []uuid.UUID
			for pIdx, photoURL := range spec.PhotoURLs {
				deterministicMediaID := uuid.MustParse(fmt.Sprintf("40000000-0000-4000-8000-00000000%02d%02d", idx+1, pIdx+1))
				var actualMediaID uuid.UUID
				err := conn.QueryRow(txCtx, `
					INSERT INTO media (id, s3_key, content_type, size_bytes, width, height, status, uploaded_by)
					VALUES ($1, $2, 'image/jpeg', 420000, 1200, 800, 'uploaded', $3)
					ON CONFLICT (s3_key) DO UPDATE
					SET status = 'uploaded'
					RETURNING id
				`, deterministicMediaID, photoURL, spec.HostID).Scan(&actualMediaID)
				if err != nil {
					return fmt.Errorf("seed media for %s: %w", spec.Title, err)
				}
				mediaIDs = append(mediaIDs, actualMediaID)
			}

			// Insert listing shell first (with NULL pointers to satisfy circular FK)
			_, err := conn.Exec(txCtx, `
				INSERT INTO listings (id, host_id, status)
				VALUES ($1, $2, 'live')
				ON CONFLICT (id) DO UPDATE
				SET host_id = EXCLUDED.host_id,
				    status = 'live',
				    updated_at = now()
			`, spec.ListingID, spec.HostID)
			if err != nil {
				return fmt.Errorf("seed listing %s: %w", spec.Title, err)
			}

			// Insert approved live listing_version
			_, err = conn.Exec(txCtx, `
				INSERT INTO listing_versions (
					id, listing_id, version_no, status, wizard_step,
					property_type, title, description,
					address_line1, city, region, country, postal_code,
					timezone, max_guests, base_price_minor, cleaning_fee_minor, currency,
					instant_book, check_in_time, check_out_time,
					house_rules, cancellation_policy, amenities, media_ids,
					submitted_at, decided_at
				) VALUES (
					$1, $2, 1, 'approved', 6,
					$3, $4, $5,
					$6, $7, $8, $9, $10,
					$11, $12, $13, $14, $15,
					$16, $17::time, $18::time,
					$19, $20, $21, $22,
					$23, $23
				)
				ON CONFLICT (id) DO UPDATE
				SET status = 'approved',
				    title = EXCLUDED.title,
				    description = EXCLUDED.description,
				    base_price_minor = EXCLUDED.base_price_minor,
				    cleaning_fee_minor = EXCLUDED.cleaning_fee_minor,
				    instant_book = EXCLUDED.instant_book,
				    cancellation_policy = EXCLUDED.cancellation_policy,
				    amenities = EXCLUDED.amenities,
				    media_ids = EXCLUDED.media_ids,
				    updated_at = now()
			`,
				spec.VersionID, spec.ListingID,
				spec.PropertyType, spec.Title, spec.Description,
				spec.AddressLine1, spec.City, spec.Region, spec.Country, spec.PostalCode,
				spec.Timezone, spec.MaxGuests, spec.BasePriceMinor, spec.CleaningFeeMinor, spec.Currency,
				spec.InstantBook, spec.CheckInTime, spec.CheckOutTime,
				spec.HouseRules, spec.CancellationPolicy, spec.Amenities, mediaIDs,
				now,
			)
			if err != nil {
				return fmt.Errorf("seed listing_version %s: %w", spec.Title, err)
			}

			// Link live_version_id and working_version_id on listings
			_, err = conn.Exec(txCtx, `
				UPDATE listings
				SET live_version_id = $2,
				    working_version_id = $2,
				    status = 'live',
				    updated_at = now()
				WHERE id = $1
			`, spec.ListingID, spec.VersionID)
			if err != nil {
				return fmt.Errorf("link live_version_id for %s: %w", spec.Title, err)
			}

			// Insert passing check results
			_, _ = conn.Exec(txCtx, `DELETE FROM listing_check_results WHERE listing_version_id = $1`, spec.VersionID)
			for _, checkCode := range []string{"min_photos", "required_fields", "house_rules_policy"} {
				_, err := conn.Exec(txCtx, `
					INSERT INTO listing_check_results (listing_version_id, check_code, outcome, detail)
					VALUES ($1, $2, 'pass', 'Automated seed verification passed')
				`, spec.VersionID, checkCode)
				if err != nil {
					return fmt.Errorf("seed check result %s: %w", checkCode, err)
				}
			}

			// Insert primary owner in listing_hosts
			_, err = conn.Exec(txCtx, `
				INSERT INTO listing_hosts (
					listing_id, user_id, role,
					can_manage_listing, can_manage_calendar, can_manage_reservations,
					can_message_guests, can_view_insights, payout_share_bps, config
				) VALUES (
					$1, $2, 'owner',
					true, true, true,
					true, true, 10000, '{"notifications_enabled":true}'::jsonb
				)
				ON CONFLICT (listing_id, user_id) DO NOTHING
			`, spec.ListingID, spec.HostID)
			if err != nil {
				return fmt.Errorf("seed owner listing_hosts: %w", err)
			}

			// Optional Co-Host
			if spec.CohostID != nil {
				_, err = conn.Exec(txCtx, `
					INSERT INTO listing_hosts (
						listing_id, user_id, role,
						can_manage_listing, can_manage_calendar, can_manage_reservations,
						can_message_guests, can_view_insights, payout_share_bps, config, invited_by
					) VALUES (
						$1, $2, 'cohost',
						false, true, true,
						true, true, 1500, '{"cohost_notes":"Handles check-ins and guest calendar"}'::jsonb, $3
					)
					ON CONFLICT (listing_id, user_id) DO NOTHING
				`, spec.ListingID, *spec.CohostID, spec.HostID)
				if err != nil {
					return fmt.Errorf("seed cohost listing_hosts: %w", err)
				}
			}
		}

		// 4. Seed Sample Reservations, Payments, Payouts & Balanced Double-Entry Ledger
		// Reservation A: Confirmed upcoming instant-book stay in Lisbon (Alfama Loft)
		resConfirmedID := uuid.MustParse("50000000-0000-4000-8000-000000000001")
		var existingCount int
		_ = conn.QueryRow(txCtx, `SELECT COUNT(1) FROM reservations WHERE id = $1`, resConfirmedID).Scan(&existingCount)
		if existingCount == 0 {
			checkInA := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 14)
			checkOutA := checkInA.AddDate(0, 0, 3) // 3 nights
			checkInAtA, _ := models.CombineLocalDateAndTime(checkInA, "15:00", "Europe/Lisbon")
			checkOutAtA, _ := models.CombineLocalDateAndTime(checkOutA, "11:00", "Europe/Lisbon")
			quoteA := models.ComputeStayQuote(14500, 3500, 3, 1400, 300)

			_, err := conn.Exec(txCtx, `
				INSERT INTO reservations (
					id, code, listing_id, listing_version_id, guest_id, host_id,
					status, is_instant_book, check_in, check_out, check_in_time, check_out_time,
					timezone, check_in_at, check_out_at, guests, guest_message,
					currency, nightly_total_minor, cleaning_fee_minor,
					guest_fee_bps, guest_fee_minor, guest_total_minor,
					host_fee_bps, host_fee_minor, host_net_minor,
					cancellation_policy, decided_at
				) VALUES (
					$1, 'SH-LISB01', $2, $3, $4, $5,
					'confirmed', true, $6, $7, '15:00'::time, '11:00'::time,
					'Europe/Lisbon', $8, $9, 2, 'Looking forward to exploring Alfama!',
					'EUR', $10, $11,
					1400, $12, $13,
					300, $14, $15,
					'moderate', $16
				)
			`,
				resConfirmedID, listingSpecs[0].ListingID, listingSpecs[0].VersionID, guestArjunID, hostInesID,
				checkInA, checkOutA, checkInAtA, checkOutAtA,
				quoteA.NightlyTotalMinor, quoteA.CleaningFeeMinor,
				quoteA.GuestFeeMinor, quoteA.GuestTotalMinor,
				quoteA.HostFeeMinor, quoteA.HostNetMinor,
				now,
			)
			if err != nil {
				return fmt.Errorf("seed confirmed reservation: %w", err)
			}

			for _, night := range models.EnumerateNights(checkInA, checkOutA) {
				_, err := conn.Exec(txCtx, `
					INSERT INTO listing_calendar (listing_id, night, status, reservation_id)
					VALUES ($1, $2, 'booked', $3)
					ON CONFLICT (listing_id, night) DO NOTHING
				`, listingSpecs[0].ListingID, night, resConfirmedID)
				if err != nil {
					return fmt.Errorf("seed calendar for confirmed reservation: %w", err)
				}
			}

			payAID := uuid.MustParse("60000000-0000-4000-8000-000000000001")
			_, err = conn.Exec(txCtx, `
				INSERT INTO payments (id, reservation_id, kind, status, amount_minor, currency, idempotency_key, simulated_ref)
				VALUES ($1, $2, 'charge', 'succeeded', $3, 'EUR', 'seed:charge:SH-LISB01', 'sim_chg_seed_lisb01')
			`, payAID, resConfirmedID, quoteA.GuestTotalMinor)
			if err != nil {
				return fmt.Errorf("seed charge payment A: %w", err)
			}

			payoutAID := uuid.MustParse("70000000-0000-4000-8000-000000000001")
			_, err = conn.Exec(txCtx, `
				INSERT INTO payouts (id, reservation_id, host_id, status, gross_minor, host_fee_minor, net_minor, currency, idempotency_key)
				VALUES ($1, $2, $3, 'scheduled', $4, $5, $6, 'EUR', 'seed:payout:SH-LISB01')
			`, payoutAID, resConfirmedID, hostInesID, quoteA.AccommodationSubtotalMinor, quoteA.HostFeeMinor, quoteA.HostNetMinor)
			if err != nil {
				return fmt.Errorf("seed payout A: %w", err)
			}

			legsA := services.BuildChargeLedgerEntries(
				uuid.MustParse("80000000-0000-4000-8000-000000000001"),
				resConfirmedID, payAID, "EUR",
				quoteA.GuestTotalMinor, quoteA.GuestFeeMinor, quoteA.HostFeeMinor, quoteA.HostNetMinor,
			)
			for _, leg := range legsA {
				_, err := conn.Exec(txCtx, `
					INSERT INTO ledger_entries (txn_id, reservation_id, payment_id, account, direction, amount_minor, currency)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
				`, leg.TxnID, leg.ReservationID, leg.PaymentID, leg.Account, string(leg.Direction), leg.AmountMinor, leg.Currency)
				if err != nil {
					return fmt.Errorf("seed ledger A: %w", err)
				}
			}
		}

		// Reservation B: Pending 24h request-to-book in Lisbon (Príncipe Real Townhouse)
		resPendingID := uuid.MustParse("50000000-0000-4000-8000-000000000002")
		_ = conn.QueryRow(txCtx, `SELECT COUNT(1) FROM reservations WHERE id = $1`, resPendingID).Scan(&existingCount)
		if existingCount == 0 {
			checkInB := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 21)
			checkOutB := checkInB.AddDate(0, 0, 4) // 4 nights
			checkInAtB, _ := models.CombineLocalDateAndTime(checkInB, "15:00", "Europe/Lisbon")
			checkOutAtB, _ := models.CombineLocalDateAndTime(checkOutB, "11:00", "Europe/Lisbon")
			expiresAtB := now.Add(24 * time.Hour)
			quoteB := models.ComputeStayQuote(21000, 4500, 4, 1400, 300)

			_, err := conn.Exec(txCtx, `
				INSERT INTO reservations (
					id, code, listing_id, listing_version_id, guest_id, host_id,
					status, is_instant_book, check_in, check_out, check_in_time, check_out_time,
					timezone, check_in_at, check_out_at, guests, guest_message,
					currency, nightly_total_minor, cleaning_fee_minor,
					guest_fee_bps, guest_fee_minor, guest_total_minor,
					host_fee_bps, host_fee_minor, host_net_minor,
					cancellation_policy, request_expires_at
				) VALUES (
					$1, 'SH-REQ002', $2, $3, $4, $5,
					'pending', false, $6, $7, '15:00'::time, '11:00'::time,
					'Europe/Lisbon', $8, $9, 4, 'Hi Inês! Celebrating our anniversary in Lisbon and would love to stay at your townhouse.',
					'EUR', $10, $11,
					1400, $12, $13,
					300, $14, $15,
					'flexible', $16
				)
			`,
				resPendingID, listingSpecs[1].ListingID, listingSpecs[1].VersionID, guestClaireID, hostInesID,
				checkInB, checkOutB, checkInAtB, checkOutAtB,
				quoteB.NightlyTotalMinor, quoteB.CleaningFeeMinor,
				quoteB.GuestFeeMinor, quoteB.GuestTotalMinor,
				quoteB.HostFeeMinor, quoteB.HostNetMinor,
				expiresAtB,
			)
			if err != nil {
				return fmt.Errorf("seed pending reservation: %w", err)
			}

			for _, night := range models.EnumerateNights(checkInB, checkOutB) {
				_, err := conn.Exec(txCtx, `
					INSERT INTO listing_calendar (listing_id, night, status, reservation_id)
					VALUES ($1, $2, 'booked', $3)
					ON CONFLICT (listing_id, night) DO NOTHING
				`, listingSpecs[1].ListingID, night, resPendingID)
				if err != nil {
					return fmt.Errorf("seed calendar for pending reservation: %w", err)
				}
			}

			payBID := uuid.MustParse("60000000-0000-4000-8000-000000000002")
			_, err = conn.Exec(txCtx, `
				INSERT INTO payments (id, reservation_id, kind, status, amount_minor, currency, idempotency_key, simulated_ref)
				VALUES ($1, $2, 'charge', 'succeeded', $3, 'EUR', 'seed:charge:SH-REQ002', 'sim_chg_seed_req002')
			`, payBID, resPendingID, quoteB.GuestTotalMinor)
			if err != nil {
				return fmt.Errorf("seed charge payment B: %w", err)
			}

			legsB := services.BuildChargeLedgerEntries(
				uuid.MustParse("80000000-0000-4000-8000-000000000002"),
				resPendingID, payBID, "EUR",
				quoteB.GuestTotalMinor, quoteB.GuestFeeMinor, quoteB.HostFeeMinor, quoteB.HostNetMinor,
			)
			for _, leg := range legsB {
				_, err := conn.Exec(txCtx, `
					INSERT INTO ledger_entries (txn_id, reservation_id, payment_id, account, direction, amount_minor, currency)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
				`, leg.TxnID, leg.ReservationID, leg.PaymentID, leg.Account, string(leg.Direction), leg.AmountMinor, leg.Currency)
				if err != nil {
					return fmt.Errorf("seed ledger B: %w", err)
				}
			}
		}

		// Reservation C: Completed past stay in Kyoto with released payout
		resCompletedID := uuid.MustParse("50000000-0000-4000-8000-000000000003")
		_ = conn.QueryRow(txCtx, `SELECT COUNT(1) FROM reservations WHERE id = $1`, resCompletedID).Scan(&existingCount)
		if existingCount == 0 {
			checkInC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -10)
			checkOutC := checkInC.AddDate(0, 0, 3) // 3 nights
			checkInAtC, _ := models.CombineLocalDateAndTime(checkInC, "15:00", "Asia/Tokyo")
			checkOutAtC, _ := models.CombineLocalDateAndTime(checkOutC, "10:00", "Asia/Tokyo")
			releaseAtC := checkInAtC.Add(24 * time.Hour)
			quoteC := models.ComputeStayQuote(24000, 5000, 3, 1400, 300)

			_, err := conn.Exec(txCtx, `
				INSERT INTO reservations (
					id, code, listing_id, listing_version_id, guest_id, host_id,
					status, is_instant_book, check_in, check_out, check_in_time, check_out_time,
					timezone, check_in_at, check_out_at, guests, guest_message,
					currency, nightly_total_minor, cleaning_fee_minor,
					guest_fee_bps, guest_fee_minor, guest_total_minor,
					host_fee_bps, host_fee_minor, host_net_minor,
					cancellation_policy, decided_at, checked_in_at, completed_at
				) VALUES (
					$1, 'SH-KYO003', $2, $3, $4, $5,
					'completed', false, $6, $7, '15:00'::time, '10:00'::time,
					'Asia/Tokyo', $8, $9, 2, 'Excited to experience a traditional Kyoto machiya.',
					'EUR', $10, $11,
					1400, $12, $13,
					300, $14, $15,
					'strict', $8, $8, $9
				)
			`,
				resCompletedID, listingSpecs[4].ListingID, listingSpecs[4].VersionID, guestClaireID, hostKenjiID,
				checkInC, checkOutC, checkInAtC, checkOutAtC,
				quoteC.NightlyTotalMinor, quoteC.CleaningFeeMinor,
				quoteC.GuestFeeMinor, quoteC.GuestTotalMinor,
				quoteC.HostFeeMinor, quoteC.HostNetMinor,
			)
			if err != nil {
				return fmt.Errorf("seed completed reservation: %w", err)
			}

			payCID := uuid.MustParse("60000000-0000-4000-8000-000000000003")
			_, err = conn.Exec(txCtx, `
				INSERT INTO payments (id, reservation_id, kind, status, amount_minor, currency, idempotency_key, simulated_ref)
				VALUES ($1, $2, 'charge', 'succeeded', $3, 'EUR', 'seed:charge:SH-KYO003', 'sim_chg_seed_kyo003')
			`, payCID, resCompletedID, quoteC.GuestTotalMinor)
			if err != nil {
				return fmt.Errorf("seed charge payment C: %w", err)
			}

			payoutCID := uuid.MustParse("70000000-0000-4000-8000-000000000003")
			_, err = conn.Exec(txCtx, `
				INSERT INTO payouts (id, reservation_id, host_id, status, gross_minor, host_fee_minor, net_minor, currency, release_at, released_at, idempotency_key, simulated_ref)
				VALUES ($1, $2, $3, 'released', $4, $5, $6, 'EUR', $7, $7, 'seed:payout:SH-KYO003', 'sim_po_seed_kyo003')
			`, payoutCID, resCompletedID, hostKenjiID, quoteC.AccommodationSubtotalMinor, quoteC.HostFeeMinor, quoteC.HostNetMinor, releaseAtC)
			if err != nil {
				return fmt.Errorf("seed payout C: %w", err)
			}

			legsCCharge := services.BuildChargeLedgerEntries(
				uuid.MustParse("80000000-0000-4000-8000-000000000003"),
				resCompletedID, payCID, "EUR",
				quoteC.GuestTotalMinor, quoteC.GuestFeeMinor, quoteC.HostFeeMinor, quoteC.HostNetMinor,
			)
			for _, leg := range legsCCharge {
				_, err := conn.Exec(txCtx, `
					INSERT INTO ledger_entries (txn_id, reservation_id, payment_id, account, direction, amount_minor, currency)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
				`, leg.TxnID, leg.ReservationID, leg.PaymentID, leg.Account, string(leg.Direction), leg.AmountMinor, leg.Currency)
				if err != nil {
					return fmt.Errorf("seed charge ledger C: %w", err)
				}
			}

			legsCPayout := services.BuildPayoutReleaseLedgerEntries(
				uuid.MustParse("80000000-0000-4000-8000-000000000004"),
				resCompletedID, payoutCID, "EUR",
				quoteC.HostNetMinor,
			)
			for _, leg := range legsCPayout {
				_, err := conn.Exec(txCtx, `
					INSERT INTO ledger_entries (txn_id, reservation_id, payout_id, account, direction, amount_minor, currency)
					VALUES ($1, $2, $3, $4, $5, $6, $7)
				`, leg.TxnID, leg.ReservationID, leg.PayoutID, leg.Account, string(leg.Direction), leg.AmountMinor, leg.Currency)
				if err != nil {
					return fmt.Errorf("seed payout ledger C: %w", err)
				}
			}

			afterBytes, _ := json.Marshal(map[string]any{
				"code":      "SH-KYO003",
				"status":    enums.ReservationStatusCompleted,
				"net_minor": quoteC.HostNetMinor,
			})
			_, err = conn.Exec(txCtx, `
				INSERT INTO event_log (occurred_at, actor_type, actor_id, action, object_type, object_id, after_state, request_id)
				VALUES ($1, 'system', NULL, 'payout.release', 'payout', $2, $3, 'seed-init')
			`, releaseAtC, payoutCID, string(afterBytes))
			if err != nil {
				return fmt.Errorf("seed event_log C: %w", err)
			}
		}

		// 6. Seed Conversations & Messages for Reservations
		type seedMsg struct {
			id        uuid.UUID
			senderID  uuid.UUID
			body      string
			createdAt time.Time
		}
		type seedConv struct {
			id            uuid.UUID
			reservationID uuid.UUID
			listingID     uuid.UUID
			guestID       uuid.UUID
			hostID        uuid.UUID
			createdAt     time.Time
			messages      []seedMsg
		}

		convSeeds := []seedConv{
			{
				id:            uuid.MustParse("90000000-0000-4000-8000-000000000001"),
				reservationID: resConfirmedID,
				listingID:     listingSpecs[0].ListingID,
				guestID:       guestArjunID,
				hostID:        hostInesID,
				createdAt:     now.Add(-5 * time.Hour),
				messages: []seedMsg{
					{
						id:        uuid.MustParse("91000000-0000-4000-8000-000000000001"),
						senderID:  guestArjunID,
						body:      "Looking forward to exploring Alfama! What is the best way to get from Lisbon airport to the loft?",
						createdAt: now.Add(-5 * time.Hour),
					},
					{
						id:        uuid.MustParse("91000000-0000-4000-8000-000000000002"),
						senderID:  hostInesID,
						body:      "Hi Arjun! The easiest route is the metro (red line, change to blue at São Sebastião down to Santa Apolónia) or a quick 20-minute taxi ride. Safe travels!",
						createdAt: now.Add(-4 * time.Hour),
					},
				},
			},
			{
				id:            uuid.MustParse("90000000-0000-4000-8000-000000000002"),
				reservationID: resPendingID,
				listingID:     listingSpecs[1].ListingID,
				guestID:       guestClaireID,
				hostID:        hostInesID,
				createdAt:     now.Add(-2 * time.Hour),
				messages: []seedMsg{
					{
						id:        uuid.MustParse("91000000-0000-4000-8000-000000000003"),
						senderID:  guestClaireID,
						body:      "Hi Inês! Celebrating our anniversary in Lisbon and would love to stay at your townhouse.",
						createdAt: now.Add(-2 * time.Hour),
					},
					{
						id:        uuid.MustParse("91000000-0000-4000-8000-000000000004"),
						senderID:  hostInesID,
						body:      "Hi Claire! Congratulations on your anniversary! The townhouse is perfect for a celebration. Let me know if you need any romantic restaurant recommendations in Príncipe Real.",
						createdAt: now.Add(-1 * time.Hour),
					},
				},
			},
			{
				id:            uuid.MustParse("90000000-0000-4000-8000-000000000003"),
				reservationID: resCompletedID,
				listingID:     listingSpecs[4].ListingID,
				guestID:       guestClaireID,
				hostID:        hostKenjiID,
				createdAt:     now.Add(-10 * 24 * time.Hour),
				messages: []seedMsg{
					{
						id:        uuid.MustParse("91000000-0000-4000-8000-000000000005"),
						senderID:  guestClaireID,
						body:      "Excited to experience a traditional Kyoto machiya.",
						createdAt: now.Add(-10 * 24 * time.Hour),
					},
					{
						id:        uuid.MustParse("91000000-0000-4000-8000-000000000006"),
						senderID:  hostKenjiID,
						body:      "Welcome Claire! The machiya is fully prepared, and fresh cedar tea awaits you upon check-in. Let me know if you need help with temple guides.",
						createdAt: now.Add(-10*24*time.Hour + 30*time.Minute),
					},
				},
			},
		}

		for _, cs := range convSeeds {
			_, err := conn.Exec(txCtx, `
				INSERT INTO conversations (id, reservation_id, listing_id, guest_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (reservation_id) DO UPDATE
				SET updated_at = EXCLUDED.updated_at
			`, cs.id, cs.reservationID, cs.listingID, cs.guestID, cs.createdAt, now)
			if err != nil {
				return fmt.Errorf("seed conversation (%s): %w", cs.reservationID, err)
			}

			for _, m := range cs.messages {
				_, err := conn.Exec(txCtx, `
					INSERT INTO messages (id, conversation_id, sender_id, body, media_ids, flags, created_at)
					VALUES ($1, $2, $3, $4, ARRAY[]::uuid[], ARRAY[]::text[], $5)
					ON CONFLICT (id) DO NOTHING
				`, m.id, cs.id, m.senderID, m.body, m.createdAt)
				if err != nil {
					return fmt.Errorf("seed message (%s): %w", m.id, err)
				}
			}

			// Seed conversation read receipts
			_, _ = conn.Exec(txCtx, `
				INSERT INTO conversation_reads (conversation_id, user_id, last_read_at)
				VALUES ($1, $2, $3)
				ON CONFLICT (conversation_id, user_id) DO UPDATE
				SET last_read_at = EXCLUDED.last_read_at
			`, cs.id, cs.guestID, now)

			_, _ = conn.Exec(txCtx, `
				INSERT INTO conversation_reads (conversation_id, user_id, last_read_at)
				VALUES ($1, $2, $3)
				ON CONFLICT (conversation_id, user_id) DO UPDATE
				SET last_read_at = EXCLUDED.last_read_at
			`, cs.id, cs.hostID, now)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	fmt.Println("=======================================================================")
	fmt.Println("StayHub V1 Seed Completed Successfully!")
	fmt.Println("-----------------------------------------------------------------------")
	fmt.Println("Sample Accounts (Password for all accounts: Password123!):")
	fmt.Println("  Admin : admin@stayhub.dev")
	fmt.Println("  Host 1: ines.costa@stayhub.dev       (Lisbon & Goa owner)")
	fmt.Println("  Host 2: kenji.takahashi@stayhub.dev  (Kyoto owner & Lisbon co-host)")
	fmt.Println("  Guest : claire.dubois@stayhub.dev")
	fmt.Println("  Guest : arjun.mehta@stayhub.dev")
	fmt.Println("-----------------------------------------------------------------------")
	fmt.Println("Seeded Markets & Live Listings:")
	fmt.Println("  1. Lisbon (PT) - Alfama Terracotta Loft (Instant Book, Moderate, €145/n)")
	fmt.Println("  2. Lisbon (PT) - Príncipe Real Townhouse (Request Book, Flexible, €210/n)")
	fmt.Println("  3. Goa    (IN) - Assagao Portuguese Villa (Instant Book, Firm, €180/n)")
	fmt.Println("  4. Goa    (IN) - Morjim Beachfront Bungalow (Request Book, Moderate, €115/n)")
	fmt.Println("  5. Kyoto  (JP) - Higashiyama Cedar Machiya (Request Book, Strict, €240/n)")
	fmt.Println("  6. Kyoto  (JP) - Kamogawa Riverfront Studio (Instant Book, Flexible, €130/n)")
	fmt.Println("=======================================================================")
}
