create schema if not exists audit;

create table audit.audit_logs
(
    id            uuid primary key,
    occurred_at   timestamptz not null default now(),
    actor_type    text        not null check (actor_type in ('CUSTOMER', 'STAFF', 'MERCHANT', 'SYSTEM')),
    actor_id      uuid        null,
    action        text        not null,
    resource_type text        not null,
    resource_id   text        not null,
    outcome       text        not null check (outcome in ('SUCCESS', 'DENIED', 'ERROR')),
    reason        text        null,
    request_id    text        not null,
    trace_id      text        not null,
    ip            text        null,
    user_agent    text        null,
    metadata      jsonb       null
);

create index audit_logs_resource_idx on audit.audit_logs (resource_type, resource_id, occurred_at);
create index audit_logs_actor_idx on audit.audit_logs (actor_id, occurred_at);

comment on table audit.audit_logs is '';
comment on column audit.audit_logs.id is '';
comment on column audit.audit_logs.occurred_at is '';
comment on column audit.audit_logs.actor_type is '';
comment on column audit.audit_logs.actor_id is '';
comment on column audit.audit_logs.action is '';
comment on column audit.audit_logs.resource_type is '';
comment on column audit.audit_logs.resource_id is '';
comment on column audit.audit_logs.outcome is '';
comment on column audit.audit_logs.reason is '';
comment on column audit.audit_logs.request_id is '';
comment on column audit.audit_logs.trace_id is '';
comment on column audit.audit_logs.ip is '';
comment on column audit.audit_logs.user_agent is '';
comment on column audit.audit_logs.metadata is '';
