-- Reverts 0001_init. Not applied automatically; run manually if needed.
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS request_events;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS counters;
DROP TABLE IF EXISTS themes;
DROP TABLE IF EXISTS routing_rules;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS departments;
