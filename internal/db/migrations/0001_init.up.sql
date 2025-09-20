-- users
CREATE TABLE users (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  provider VARCHAR(32) NOT NULL,          -- "google"
  provider_id VARCHAR(191) NOT NULL,      -- sub
  email VARCHAR(191) NOT NULL,
  name VARCHAR(191) NULL,
  picture VARCHAR(512) NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  last_login_at TIMESTAMP NULL,
  UNIQUE KEY uq_provider_user (provider, provider_id),
  UNIQUE KEY uq_email (email)
);

-- user_sessions (log sesi login)
CREATE TABLE user_sessions (
  id CHAR(36) PRIMARY KEY,                -- UUID
  user_id BIGINT UNSIGNED NOT NULL,
  ip VARCHAR(64) NULL,
  user_agent VARCHAR(512) NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  ended_at TIMESTAMP NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

-- quizzes
CREATE TABLE quizzes (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(191) NULL,
  image_path VARCHAR(512) NOT NULL,
  image_hash CHAR(64) NOT NULL,
  image_width INT NULL,
  image_height INT NULL,
  ocr_text MEDIUMTEXT NULL,
  ocr_lang VARCHAR(32) NULL,
  status ENUM('uploaded','processing','completed','error') DEFAULT 'uploaded',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id),
  KEY idx_user (user_id),
  UNIQUE KEY uq_image_hash_user (user_id, image_hash)
);

-- answers (per sumber: OPENAI/CLAUDE/DEEPSEEK/MANUAL)
CREATE TABLE answers (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  quiz_id BIGINT UNSIGNED NOT NULL,
  source ENUM('OPENAI','CLAUDE','DEEPSEEK','MANUAL') NOT NULL,
  answer_text TEXT NOT NULL,
  reason_text TEXT NULL,
  score DECIMAL(5,2) NULL,
  latency_ms INT NULL,
  token_usage_json JSON NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (quiz_id) REFERENCES quizzes(id),
  UNIQUE KEY uq_quiz_source (quiz_id, source)
);

-- providers_logs (jejak request/response)
CREATE TABLE providers_logs (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  quiz_id BIGINT UNSIGNED NOT NULL,
  source ENUM('OPENAI','CLAUDE','DEEPSEEK') NOT NULL,
  status_code INT NULL,
  req_snippet TEXT NULL,
  resp_snippet TEXT NULL,
  error_text TEXT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (quiz_id) REFERENCES quizzes(id),
  KEY idx_quiz (quiz_id)
);
