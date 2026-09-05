# Database architecture decision

HireTech uses PostgreSQL as the system of record. Authentication, tenant membership, interviews, answers, evaluations, audit events, retention and deletion jobs require transactions, foreign keys and deterministic authorization predicates.

Variable model, prompt, rubric and provider metadata may use validated `JSONB` columns while retaining immutable versions and relational ownership. Large code artifacts, exports and fine-tuned model files must not be stored as PostgreSQL blobs; use an S3-compatible object store such as MinIO with database-owned metadata, checksums, tenant scope and retention state.

MongoDB is not part of the initial architecture. Introducing it would create a second authorization, retention, backup and deletion boundary without a requirement that PostgreSQL cannot satisfy.
