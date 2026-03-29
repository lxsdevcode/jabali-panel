<?php

declare(strict_types=1);

namespace App\Services;

/**
 * Legacy Roundcube identity service.
 *
 * Stalwart/Bulwark now handles all mail identities via JMAP.
 * Methods are kept as no-ops for backward compatibility.
 */
class RoundcubeIdentityService
{
    public function addIdentity(string $mailboxEmail, string $aliasEmail, string $name = ''): bool
    {
        return true;
    }

    public function removeIdentity(string $mailboxEmail, string $aliasEmail): bool
    {
        return true;
    }

    /**
     * @param  array<string>  $aliasEmails
     */
    public function syncIdentities(string $mailboxEmail, array $aliasEmails): void {}
}
