FROM postgres:16-alpine

COPY migrations/001_init.sql /docker-entrypoint-initdb.d/001_init.sql
COPY migrations/002_seed.sql /docker-entrypoint-initdb.d/002_seed.sql
