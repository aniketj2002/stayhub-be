package models

// Amenity maps to the `amenities` reference table.
type Amenity struct {
	Code     string `db:"code" json:"code"`
	Label    string `db:"label" json:"label"`
	Category string `db:"category" json:"category"`
}

// CancellationPolicy maps to the `cancellation_policies` reference table.
type CancellationPolicy struct {
	Code        string                   `db:"code" json:"code"`
	Name        string                   `db:"name" json:"name"`
	Description string                   `db:"description" json:"description"`
	Rules       []CancellationPolicyRule `db:"-" json:"rules,omitempty"`
}

// CancellationPolicyRule maps to the `cancellation_policy_rules` table.
type CancellationPolicyRule struct {
	PolicyCode        string `db:"policy_code" json:"policy_code"`
	MinHoursBefore    int    `db:"min_hours_before" json:"min_hours_before"`
	RefundPct         int    `db:"refund_pct" json:"refund_pct"`
	RefundsServiceFee bool   `db:"refunds_service_fee" json:"refunds_service_fee"`
}
