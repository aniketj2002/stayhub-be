-- =====================================================================
-- StayHub — Reference Data Seed (Amenities & Cancellation Policies)
-- =====================================================================

INSERT INTO amenities (code, label, category) VALUES
  ('wifi',                  'Fast Wi-Fi',              'essentials'),
  ('kitchen',               'Full kitchen',            'essentials'),
  ('washer',                'Washing machine',         'essentials'),
  ('dryer',                 'Dryer',                   'essentials'),
  ('air_conditioning',      'Air conditioning',        'essentials'),
  ('heating',               'Central heating',         'essentials'),
  ('dedicated_workspace',   'Dedicated workspace',     'essentials'),
  ('tv',                    'HD TV',                   'essentials'),
  ('hair_dryer',            'Hair dryer',              'essentials'),
  ('iron',                  'Iron',                    'essentials'),
  ('pool',                  'Swimming pool',           'features'),
  ('hot_tub',               'Hot tub',                 'features'),
  ('free_parking',          'Free parking on premises','features'),
  ('ev_charger',            'EV charger',              'features'),
  ('crib',                  'Crib',                    'features'),
  ('gym',                   'Gym',                     'features'),
  ('bbq_grill',             'BBQ grill',               'features'),
  ('breakfast',             'Breakfast included',      'features'),
  ('indoor_fireplace',      'Indoor fireplace',        'features'),
  ('self_check_in',         'Self check-in',           'features'),
  ('pets_allowed',          'Pets allowed',            'features'),
  ('beachfront',            'Beachfront access',       'location'),
  ('waterfront',            'Waterfront view',         'location'),
  ('smoke_alarm',           'Smoke alarm',             'safety'),
  ('carbon_monoxide_alarm', 'Carbon monoxide alarm',   'safety'),
  ('first_aid_kit',         'First aid kit',           'safety'),
  ('fire_extinguisher',     'Fire extinguisher',       'safety')
ON CONFLICT (code) DO NOTHING;

INSERT INTO cancellation_policies (code, name, description) VALUES
  ('flexible', 'Flexible', 'Full refund up to 24 hours before check-in. Non-refundable within 24 hours of check-in.'),
  ('moderate', 'Moderate', 'Full refund up to 5 days (120 hours) before check-in; 50% refund up to 24 hours before check-in.'),
  ('firm',     'Firm',     'Full refund up to 30 days (720 hours) before check-in; 50% refund up to 7 days (168 hours) before check-in.'),
  ('strict',   'Strict',   '50% refund up to 14 days (336 hours) before check-in. Non-refundable within 14 days of check-in.')
ON CONFLICT (code) DO NOTHING;

-- Rules: hours_before = floor((reservations.check_in_at - cancel_time) in hours).
-- Pick the row with the largest min_hours_before <= hours_before.
INSERT INTO cancellation_policy_rules (policy_code, min_hours_before, refund_pct, refunds_service_fee) VALUES
  -- Flexible: >= 24h -> 100% (+ service fee), < 24h -> 0%
  ('flexible', 24,  100, true),
  ('flexible', 0,   0,   false),
  -- Moderate: >= 120h (5d) -> 100% (+ service fee), >= 24h -> 50% (no service fee), < 24h -> 0%
  ('moderate', 120, 100, true),
  ('moderate', 24,  50,  false),
  ('moderate', 0,   0,   false),
  -- Firm: >= 720h (30d) -> 100% (+ service fee), >= 168h (7d) -> 50% (no service fee), < 168h -> 0%
  ('firm',     720, 100, true),
  ('firm',     168, 50,  false),
  ('firm',     0,   0,   false),
  -- Strict: >= 336h (14d) -> 50% (no service fee), < 336h -> 0%
  ('strict',   336, 50,  false),
  ('strict',   0,   0,   false)
ON CONFLICT (policy_code, min_hours_before) DO NOTHING;
