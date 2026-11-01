-- name: InsertCustomer :exec
INSERT INTO identity.customers (
    id, phone, phone_verified_at, onboarding_status, full_name, 
    date_of_birth, closed_at, created_at, updated_at, version
) 
VALUES (
    $1, $2, $3, $4, $5, 
    $6, $7, $8, $9, $10
);

-- name: GetCustomerById :one
select 
    id, phone, phone_verified_at, onboarding_status, full_name, 
    date_of_birth, closed_at, created_at, updated_at, version 
from identity.customers where id = $1;

-- name: GetCustomerByPhone :one
select 
    id, phone, phone_verified_at, onboarding_status, full_name, 
    date_of_birth, closed_at, created_at, updated_at, version 
from identity.customers where phone = $1;

-- name: CreateNewCredential :exec
INSERT INTO identity.credentials (customer_id, pin_hash, pin_failed_count, pin_locked_until, pin_hard_locked, pin_updated_at, version) 
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: AuthenCredential :one
SELECT 
    customer_id, pin_hash, pin_failed_count, pin_locked_until, pin_hard_locked, pin_updated_at, version
FROM identity.credentials WHERE customer_id = $1;

-- name: UpdatePINFailed :exec
UPDATE identity.credentials 
SET 
    pin_failed_count = pin_failed_count + 1, 
    pin_locked_until = CASE 
        WHEN pin_failed_count + 1 >= 3
            THEN NOW() + INTERNAL '15 minutes'
        ELSE pin_locked_until
    END,
    pin_hard_locked = CASE
        WHEN pin_failed_count + 1 >= 5
            THEN TRUE
        ELSE pin_hard_locked
    END,
    version = version + 1 
WHERE customer_id = $1;