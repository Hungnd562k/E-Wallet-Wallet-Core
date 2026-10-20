create schema if not exists identity;

create table identity.customers
(
    id                uuid primary key,
    phone             text unique,
    phone_verified_at timestamptz not null default now(),
    onboarding_status TEXT        not null check (onboarding_status in
                                                  ('REGISTERED', 'KYC_PENDING', 'KYC_VERIFIED', 'KYC_REJECTED',
                                                   'MANUAL_REVIEW')),
    full_name         TEXT        null,
    date_of_birth     date        null,
    closed_at         timestamptz null,
    created_at        timestamptz not null, -- add trigger
    updated_at        timestamptz not null, -- add trigger
    version           int         not null
);

comment on table identity.customers is '';
comment on column identity.customers.phone is '';
comment on column identity.customers.phone_verified_at is '';
comment on column identity.customers.onboarding_status is '';
comment on column identity.customers.full_name is '';
comment on column identity.customers.date_of_birth is '';
comment on column identity.customers.closed_at is '';
comment on column identity.customers.created_at is '';
comment on column identity.customers.updated_at is '';
comment on column identity.customers.version is '';

--

create table identity.phone_verifications
(
    id          uuid primary key,
    phone       text        not null,
    purpose     text        not null check (purpose in ('REGISTER', 'CHANGE_PHONE', 'NEW_DEVICE')),
    otp_hash    bytea       not null,
    attempts    int         not null default 0,
    expires_at  timestamptz not null,
    verified_at timestamptz null,
    constraint attempts check (attempts <= 5)
);

comment on table identity.phone_verifications is '';
comment on column identity.phone_verifications.id is '';
comment on column identity.phone_verifications.phone is '';
comment on column identity.phone_verifications.purpose is '';
comment on column identity.phone_verifications.otp_hash is '';
comment on column identity.phone_verifications.attempts is '';
comment on column identity.phone_verifications.expires_at is '';
comment on column identity.phone_verifications.verified_at is '';

--

create table identity.devices
(
    id                 uuid primary key,
    customer_id        uuid,
    device_fingerprint text,
    platform           text        null,
    model              text        null,
    app_version        text        null,
    status             text        not null check (status in ('STATUS', 'REVOKED')),
    bound_at           timestamptz null,
    revoked_at         timestamptz null,
    constraint fk_devices_customers foreign key (customer_id) references customers (id)
);

create unique index uq_devices_customer_active on identity.devices (customer_id) where status = 'ACTIVE';

comment on table identity.devices is '';
comment on column identity.devices.id is '';
comment on column identity.devices.customer_id is '';
comment on column identity.devices.device_fingerprint is '';
comment on column identity.devices.platform is '';
comment on column identity.devices.model is '';
comment on column identity.devices.app_version is '';
comment on column identity.devices.bound_at is '';
comment on column identity.devices.revoked_at is '';

--

create table identity.credentials
(
    customer_id      uuid primary key, -- fk to customer
    pin_hash         text        null, -- rang buoc argon2id
    pin_failed_count int         not null default 0,
    pin_locked_until timestamptz null,
    pin_hard_locked  boolean     not null default false,
    pin_updated_at   timestamptz null,
    version          int         not null,
    constraint fk_credentials_customers foreign key (customer_id) references customers (id)
);

comment on table identity.credentials is '';
comment on column identity.credentials.customer_id is '';
comment on column identity.credentials.pin_hash is '';
comment on column identity.credentials.pin_failed_count is '';
comment on column identity.credentials.pin_locked_until is '';
comment on column identity.credentials.pin_hard_locked is '';
comment on column identity.credentials.pin_updated_at is '';
comment on column identity.credentials.version is '';

--

create table identity.refresh_tokens
(
    id          uuid primary key,
    customer_id uuid        not null,
    device_id   uuid,
    family_id   uuid        not null,
    token_hash  bytea       not null unique,
    replaced_by uuid        null,
    expires_at  timestamptz not null,
    revoked_at  timestamptz null,
    constraint fk_refresh_token_customers foreign key (customer_id) references customers (id),
    constraint fk_refresh_token_devices foreign key (device_id) references devices (id),
    constraint fk_refresh_token_this foreign key (replaced_by) references refresh_tokens (id)
);

comment on table identity.refresh_tokens is '';
comment on column identity.refresh_tokens.id is '';
comment on column identity.refresh_tokens.customer_id is '';
comment on column identity.refresh_tokens.device_id is '';
comment on column identity.refresh_tokens.family_id is '';
comment on column identity.refresh_tokens.token_hash is '';
comment on column identity.refresh_tokens.replaced_by is '';
comment on column identity.refresh_tokens.expires_at is '';
comment on column identity.refresh_tokens.revoked_at is '';

--

create table identity.consents
(
    id             uuid primary key,
    customer_id    uuid,
    consent_type   text        not null check (consent_type in
                                               ('TERMS', 'PERSONAL_DATA', 'BIOMETRIC_DATA', 'DATA_SHARING_BANK')),
    policy_version text        not null,
    granted_at     timestamptz not null,
    withdrawn_at   timestamptz null,
    ip             text,
    user_agent     text        null,
    constraint fk_consents_customers foreign key (customer_id) references customers (id)
);

comment on table identity.consents is '';
comment on column identity.consents.id is '';
comment on column identity.consents.customer_id is '';
comment on column identity.consents.consent_type is '';
comment on column identity.consents.policy_version is '';
comment on column identity.consents.granted_at is '';
comment on column identity.consents.withdrawn_at is '';
comment on column identity.consents.ip is '';
comment on column identity.consents.user_agent is '';