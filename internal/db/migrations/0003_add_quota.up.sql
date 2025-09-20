
ALTER TABLE users
  ADD COLUMN quiz_quota INT DEFAULT 10 AFTER picture,
  ADD COLUMN quiz_used INT DEFAULT 0 AFTER quiz_quota;
