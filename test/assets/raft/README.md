This directory is reserved for local TLS test assets (CA/node certs) when generating mTLS material manually.

Notes:
- The E2E mTLS test (Test_43) generates ephemeral certificates via openssl when available and does not require pre-populated files.
- You can optionally place CA and node materials here to reuse across runs.
