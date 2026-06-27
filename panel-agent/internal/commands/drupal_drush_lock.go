package commands

import _ "embed"

// Reviewed, pinned Drupal + Drush dependency set (GH #459). Generated once from
// the pinned Drupal release tarball via `composer require drush/drush:^12`
// (drush resolved to 12.5.3); the lock pins the full graph. The installer
// writes these over the extracted tree and runs `composer install` (not
// `require`/`update`), so a tenant install can never resolve a different Drush
// or transitive set. Regenerate BOTH files when bumping drupalVersion:
//   tar xf drupal-<v>.tar.gz && cd drupal-<v> && \
//   COMPOSER_ALLOW_SUPERUSER=1 composer require drush/drush:^12 --no-progress
// then copy composer.json + composer.lock here under code review.
//
//go:embed drupal_assets/composer.json
var drupalComposerJSON []byte

//go:embed drupal_assets/composer.lock
var drupalComposerLock []byte
