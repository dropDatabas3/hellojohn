-- Borrar en orden inverso de dependencias
DROP TABLE IF EXISTS identity;
DROP TABLE IF EXISTS app_user;
DROP TABLE IF EXISTS client_version;
DROP TABLE IF EXISTS client;
DROP TABLE IF EXISTS tenant;

-- La extensión podés dejarla instalada; si querés limpiar a fondo:
-- DROP EXTENSION IF EXISTS "pgcrypto";
