-- OK: ADD COLUMN sem default (sem reescrita de tabela)
SET lock_timeout = '3s';
ALTER TABLE users ADD COLUMN score INT;
