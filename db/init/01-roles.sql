-- LOCAL DEVELOPMENT ONLY. Runs once, on first start of an empty Postgres volume.
-- Passwords come from the container environment through psql \getenv.

\getenv app_password YARI_APP_PASSWORD
\getenv zitadel_password ZITADEL_DB_PASSWORD
\getenv litellm_password LITELLM_DB_PASSWORD

-- The application role. Not a superuser, cannot bypass row-level security.
-- Migrations run as the database owner (user "yari") and grant this role what it needs.
CREATE ROLE yari_app LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE PASSWORD :'app_password';
GRANT CONNECT ON DATABASE yari TO yari_app;

-- Separate databases for the optional services, each with its own owner.
CREATE ROLE zitadel LOGIN NOSUPERUSER PASSWORD :'zitadel_password';
CREATE DATABASE zitadel OWNER zitadel;

CREATE ROLE litellm LOGIN NOSUPERUSER PASSWORD :'litellm_password';
CREATE DATABASE litellm OWNER litellm;
