create schema if not exists limits;

create table limits.usage (
    customer_id uuid not null references identity.customers(id),
    bucket text not null check(bucket in ('MONTH_GENERAL','MONTH_ESSENTIAL','DAY_OWN_ACCOUNT','DAY_OTHER')),
    period_key text not null,
    used_amount bigint not null,
    txn_count int not null,
    version int not null,
    updated_at timestamptz not null
);

create unique index usage_idx_uq on limits.usage (customer_id, bucket, period_key);

comment on table limits.usage is '';
comment on column limits.usage.customer_id is '';
comment on column limits.usage.bucket is '';
comment on column limits.usage.period_key is '';
comment on column limits.usage.used_amount is '';
comment on column limits.usage.txn_count is '';
comment on column limits.usage.version is '';
comment on column limits.usage.updated_at is '';

create table limits.consumptions (
    id uuid primary key,
    payment_id uuid references payment.payments(customer_id),
    customer_id uuid references limits.usage (customer_id),
    bucket text not null references limits.usage (bucket),
    period_key text not null references limits.usage (period_key),
    amount bigint not null,
    reverted_at timestamptz null,
    created_at timestamptz,
    constraint amount_greater_than_zero check (amount > 0)
);

comment on table limits.consumptions is '';
comment on column limits.consumptions.id is '';
comment on column limits.consumptions.payment_id is '';
comment on column limits.consumptions.customer_id is '';
comment on column limits.consumptions.bucket is '';
comment on column limits.consumptions.period_key is '';
comment on column limits.consumptions.amount is '';
comment on column limits.consumptions.reverted_at is '';
comment on column limits.consumptions.created_at is '';