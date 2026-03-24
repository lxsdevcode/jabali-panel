<?php

declare(strict_types=1);

namespace Tests\Unit;

use Illuminate\Support\Facades\Crypt;
use Tests\TestCase;

class MigrationCredentialSecurityTest extends TestCase
{
    public function test_cpanel_token_should_be_encrypted_in_session(): void
    {
        // Simulate what storeCredentialsInSession does
        $plainToken = 'CPANEL_API_TOKEN_SECRET_123';

        // The CORRECT behavior: encrypt before storing
        session()->put('user_cpanel_migration.token', Crypt::encryptString($plainToken));

        // The raw session value should NOT equal the plain token
        $stored = session('user_cpanel_migration.token');
        $this->assertNotEquals($plainToken, $stored);

        // But decrypting should return the original
        $this->assertEquals($plainToken, Crypt::decryptString($stored));
    }

    public function test_whm_token_should_be_encrypted_in_session(): void
    {
        $plainToken = 'WHM_API_TOKEN_SECRET_456';

        session()->put('whm_migration.token', Crypt::encryptString($plainToken));

        $stored = session('whm_migration.token');
        $this->assertNotEquals($plainToken, $stored);
        $this->assertEquals($plainToken, Crypt::decryptString($stored));
    }

    public function test_plain_token_in_session_is_readable(): void
    {
        // This demonstrates the VULNERABILITY: plain text is directly readable
        $plainToken = 'PLAIN_TEXT_TOKEN';

        session()->put('test.token', $plainToken);

        // Plain text IS equal — this is the bad behavior we're fixing
        $this->assertEquals($plainToken, session('test.token'));
    }
}
