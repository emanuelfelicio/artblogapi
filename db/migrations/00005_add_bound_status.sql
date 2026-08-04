-- +goose Up
-- +goose NO TRANSACTION
ALTER TYPE upload_status ADD VALUE 'BOUND';
