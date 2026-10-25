INSERT INTO platform.settings (key, value, description)
VALUES
    ('customer.min_age', '18'::jsonb,
     'BR-01: Minimum customer age'),

    ('kyc.max_attempts_per_day', '3'::jsonb,
     'BR-06: Maximum KYC attempts per day'),

    ('kyc.id_expiry_notice_days', '30'::jsonb,
     'BR-07: Days before ID expiry notification'),

    ('banklink.max_per_customer', '3'::jsonb,
     'BR-12: Maximum bank links per customer'),

    ('topup.min_amount', '10000'::jsonb,
     'BR-20: Minimum top-up amount'),

    ('transfer.min_amount', '1000'::jsonb,
     'BR-28: Minimum wallet transfer amount'),

    ('payment.min_amount', '1000'::jsonb,
     'BR-28: Minimum payment amount'),

    ('txn.max_amount_per_txn', '50000000'::jsonb,
     'BR-28: Maximum transaction amount'),

    ('limit.monthly_general', '100000000'::jsonb,
     'BR-25: General monthly transaction limit'),

    ('limit.monthly_with_essential', '300000000'::jsonb,
     'BR-26: Monthly transaction limit with essential services'),

    ('limit.timezone', '"Asia/Ho_Chi_Minh"'::jsonb,
     'BR-27: Timezone used for transaction limits'),

    ('auth.strong_single_threshold', '10000000'::jsonb,
     'BR-36: Single transaction threshold for strong authentication'),

    ('auth.strong_daily_threshold', '20000000'::jsonb,
     'BR-36: Daily threshold for strong authentication'),

    ('auth.new_device_daily_limit', '10000000'::jsonb,
     'BR-38: Daily transaction limit for a new device'),

    ('auth.new_device_window_hours', '24'::jsonb,
     'BR-38: New device restriction window in hours'),

    ('auth.pin_max_failures', '5'::jsonb,
     'BR-39: Maximum consecutive PIN failures'),

    ('auth.pin_lock_minutes', '30'::jsonb,
     'BR-39: PIN lock duration in minutes'),

    ('auth.challenge_ttl_seconds', '300'::jsonb,
     'BR-40: Authentication challenge TTL in seconds'),

    ('payment.pending_max_hours', '24'::jsonb,
     'BR-46: Maximum pending payment duration in hours'),

    ('refund.window_days', '90'::jsonb,
     'BR-48: Refund eligibility window in days'),

    ('merchant.callback_retry_max_hours', '24'::jsonb,
     'BR-51: Maximum merchant callback retry period in hours'),

    ('qr.ttl_minutes', '15'::jsonb,
     'BR-52: QR code expiration time in minutes'),

    ('dispute.submit_window_days', '60'::jsonb,
     'BR-60: Dispute submission window in days'),

    ('dispute.resolve_business_days', '15'::jsonb,
     'BR-60: Dispute resolution target in business days'),

    ('wallet.dormant_close_months', '12'::jsonb,
     'BR-63: Dormant wallet closure period in months'),

    ('wallet.dormant_notice_days', '30'::jsonb,
     'BR-63: Notice period before dormant wallet closure'),

    ('retention.kyc_years_after_close', '5'::jsonb,
     'BR-73: KYC retention years after account closure'),

    ('retention.ledger_years', '10'::jsonb,
     'BR-73: Ledger retention period in years'),

    ('aml.ewire_report_threshold', '500000000'::jsonb,
     'BR-74: E-wire reporting threshold');