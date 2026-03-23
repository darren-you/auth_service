CREATE TABLE IF NOT EXISTS `auth_tenants` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_key` VARCHAR(64) NOT NULL,
  `name` VARCHAR(191) NOT NULL,
  `enabled` TINYINT(1) NOT NULL DEFAULT 1,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_auth_tenants_tenant_key` (`tenant_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `auth_provider_configs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `provider` VARCHAR(32) NOT NULL,
  `client_type` VARCHAR(32) NOT NULL,
  `enabled` TINYINT(1) NOT NULL DEFAULT 1,
  `app_id` VARCHAR(191) NOT NULL DEFAULT '',
  `app_secret` TEXT NOT NULL,
  `redirect_uri` TEXT NOT NULL,
  `scope` VARCHAR(128) NOT NULL DEFAULT '',
  `team_id` VARCHAR(64) NOT NULL DEFAULT '',
  `client_id` VARCHAR(191) NOT NULL DEFAULT '',
  `key_id` VARCHAR(64) NOT NULL DEFAULT '',
  `signing_key` LONGTEXT NOT NULL,
  `test_phone` VARCHAR(32) NOT NULL DEFAULT '',
  `test_captcha` VARCHAR(32) NOT NULL DEFAULT '',
  `test_captcha_key` VARCHAR(64) NOT NULL DEFAULT '',
  `extra_json` LONGTEXT NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_auth_provider_configs_tenant_provider_client` (`tenant_id`, `provider`, `client_type`),
  KEY `idx_auth_provider_configs_tenant_id` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `auth_users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `display_name` VARCHAR(191) NOT NULL DEFAULT '',
  `avatar_url` TEXT NOT NULL,
  `role` VARCHAR(32) NOT NULL DEFAULT 'user',
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `last_login_at` TIMESTAMP NULL DEFAULT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_auth_users_tenant_id` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `auth_identities` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `auth_user_id` BIGINT UNSIGNED NOT NULL,
  `provider` VARCHAR(32) NOT NULL,
  `client_type` VARCHAR(32) NOT NULL DEFAULT '',
  `provider_subject` VARCHAR(191) NOT NULL,
  `union_id` VARCHAR(191) NOT NULL DEFAULT '',
  `profile_json` LONGTEXT NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_auth_identities_tenant_provider_subject` (`tenant_id`, `provider`, `provider_subject`),
  KEY `idx_auth_identities_tenant_id` (`tenant_id`),
  KEY `idx_auth_identities_auth_user_id` (`auth_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `auth_sessions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL,
  `auth_user_id` BIGINT UNSIGNED NOT NULL,
  `provider` VARCHAR(32) NOT NULL DEFAULT '',
  `client_type` VARCHAR(32) NOT NULL DEFAULT '',
  `refresh_token_hash` VARCHAR(64) NOT NULL,
  `expires_at` TIMESTAMP NOT NULL,
  `revoked_at` TIMESTAMP NULL DEFAULT NULL,
  `last_seen_at` TIMESTAMP NULL DEFAULT NULL,
  `metadata_json` LONGTEXT NOT NULL,
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_auth_sessions_refresh_token_hash` (`refresh_token_hash`),
  KEY `idx_auth_sessions_tenant_id` (`tenant_id`),
  KEY `idx_auth_sessions_auth_user_id` (`auth_user_id`),
  KEY `idx_auth_sessions_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
