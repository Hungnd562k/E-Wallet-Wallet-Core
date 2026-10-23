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