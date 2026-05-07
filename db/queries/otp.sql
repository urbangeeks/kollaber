-- name: CreateOTPCode :one
INSERT INTO otp_codes (email, code) VALUES ($1, $2) RETURNING *;

-- name: GetValidOTPCode :one
SELECT * FROM otp_codes
WHERE email = $1 AND code = $2 AND used_at IS NULL AND expires_at > NOW()
LIMIT 1;

-- name: MarkOTPCodeUsed :exec
UPDATE otp_codes SET used_at = NOW() WHERE id = $1;

-- name: DeleteOTPCodesByEmail :exec
DELETE FROM otp_codes WHERE email = $1;
