<?php

declare(strict_types=1);

namespace App\Services\Migration;

class MigrationEmailProvisionResult
{
    /**
     * @param  array<int, string>  $domainsEnabled  Domain names that were enabled for email
     * @param  array<int, array{email: string, password: string}>  $mailboxes  Created mailboxes with temp passwords
     * @param  array<int, string>  $forwarders  Created forwarder email addresses
     * @param  array<int, string>  $errors  Error messages from failed operations
     */
    public function __construct(
        public array $domainsEnabled = [],
        public array $mailboxes = [],
        public array $forwarders = [],
        public array $errors = [],
    ) {}
}
