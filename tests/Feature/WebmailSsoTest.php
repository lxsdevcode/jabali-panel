<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\Domain;
use App\Models\EmailDomain;
use App\Models\Mailbox;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Crypt;
use Tests\TestCase;

class WebmailSsoTest extends TestCase
{
    use RefreshDatabase;

    private string $ssoDir;

    protected function setUp(): void
    {
        parent::setUp();

        $this->ssoDir = sys_get_temp_dir().'/jabali-test-sso-tokens';
        @mkdir($this->ssoDir, 0700, true);
        config(['jabali.sso_token_dir' => $this->ssoDir]);
    }

    protected function tearDown(): void
    {
        // Clean up token files
        foreach (glob("{$this->ssoDir}/bulwark_sso_*") as $file) {
            @unlink($file);
        }
        @rmdir($this->ssoDir);

        parent::tearDown();
    }

    private function createMailboxWithPassword(User $user): Mailbox
    {
        $domain = Domain::factory()->create(['user_id' => $user->id]);
        $emailDomain = EmailDomain::create([
            'domain_name' => $domain->domain,
            'domain_id' => $domain->id,
            'user_id' => $user->id,
            'is_active' => true,
        ]);

        return Mailbox::create([
            'email_domain_id' => $emailDomain->id,
            'user_id' => $user->id,
            'local_part' => 'test',
            'password_hash' => bcrypt('secret'),
            'password_encrypted' => Crypt::encryptString('secret'),
            'is_active' => true,
        ]);
    }

    public function test_sso_redirect_uses_request_host_not_config_hostname(): void
    {
        $user = User::factory()->create();
        $mailbox = $this->createMailboxWithPassword($user);

        // When the request host differs from config hostname, SSO must follow the request
        $response = $this->actingAs($user)->get(route('webmail.sso', $mailbox));

        $response->assertRedirect();
        $redirectUrl = $response->headers->get('Location');

        // Redirect must use the same host as the request, and include a valid token
        $this->assertStringContainsString('/webmail/api/auth/sso?token=', $redirectUrl);
        // The host in the redirect must match what request()->getHost() returns (localhost in tests)
        $this->assertStringContainsString('https://localhost/', $redirectUrl);
        // Crucially: it must NOT use jabali.panel.hostname config when it differs from request host
        config(['jabali.panel.hostname' => 'different-host.example.com']);
        // Verify the route code does NOT use config — re-request should still use request host
        $response2 = $this->actingAs($user)->get(route('webmail.sso', $mailbox));
        $redirectUrl2 = $response2->headers->get('Location');
        $this->assertStringNotContainsString('different-host.example.com', $redirectUrl2);
    }

    public function test_sso_redirect_uses_hostname_when_accessed_by_hostname(): void
    {
        $user = User::factory()->create();
        $mailbox = $this->createMailboxWithPassword($user);

        // Even if jabali.panel.hostname is set, the SSO redirect should use request host
        config(['jabali.panel.hostname' => 'override.example.com']);

        $response = $this->actingAs($user)->get(route('webmail.sso', $mailbox));

        $response->assertRedirect();
        $redirectUrl = $response->headers->get('Location');

        // Must NOT use the config hostname — must use request()->getHost()
        $this->assertStringNotContainsString('override.example.com', $redirectUrl);
    }

    public function test_sso_rejects_mailbox_not_owned_by_user(): void
    {
        $owner = User::factory()->create();
        $other = User::factory()->create();
        $mailbox = $this->createMailboxWithPassword($owner);

        $response = $this->actingAs($other)
            ->get(route('webmail.sso', $mailbox));

        $response->assertForbidden();
    }

    public function test_sso_requires_authentication(): void
    {
        $user = User::factory()->create();
        $mailbox = $this->createMailboxWithPassword($user);

        $response = $this->get(route('webmail.sso', $mailbox));

        $response->assertRedirect(); // Redirect to login
    }

    public function test_sso_creates_token_file(): void
    {
        $user = User::factory()->create();
        $mailbox = $this->createMailboxWithPassword($user);

        $existingTokens = glob("{$this->ssoDir}/bulwark_sso_*");

        $response = $this->actingAs($user)
            ->get(route('webmail.sso', $mailbox));

        $response->assertRedirect();

        // A new token file should exist
        $newTokens = glob("{$this->ssoDir}/bulwark_sso_*");
        $this->assertGreaterThan(count($existingTokens), count($newTokens), 'SSO token file was not created');

        // tearDown handles cleanup
    }

    public function test_sso_token_contains_correct_data(): void
    {
        $user = User::factory()->create();
        $mailbox = $this->createMailboxWithPassword($user);

        $existingTokens = glob("{$this->ssoDir}/bulwark_sso_*");

        $response = $this->actingAs($user)
            ->get(route('webmail.sso', $mailbox));

        $response->assertRedirect();

        $newTokens = array_diff(glob("{$this->ssoDir}/bulwark_sso_*"), $existingTokens);
        $this->assertNotEmpty($newTokens);

        $tokenData = json_decode(file_get_contents(reset($newTokens)), true);
        $this->assertEquals($mailbox->email, $tokenData['email']);
        $this->assertEquals('secret', $tokenData['password']);
        $this->assertTrue($tokenData['use_direct']);
        $this->assertGreaterThan(time() - 5, $tokenData['expires']);

        // tearDown handles cleanup
    }

    public function test_sso_shows_password_required_when_no_plain_password(): void
    {
        $user = User::factory()->create();
        $domain = Domain::factory()->create(['user_id' => $user->id]);
        $emailDomain = EmailDomain::create([
            'domain_name' => $domain->domain,
            'domain_id' => $domain->id,
            'user_id' => $user->id,
            'is_active' => true,
        ]);

        $mailbox = Mailbox::create([
            'email_domain_id' => $emailDomain->id,
            'user_id' => $user->id,
            'local_part' => 'nopw',
            'password_hash' => bcrypt('secret'),
            'password_encrypted' => null,
            'is_active' => true,
        ]);

        $response = $this->actingAs($user)
            ->get(route('webmail.sso', $mailbox));

        $response->assertOk();
        $response->assertViewIs('webmail-password-required');
    }
}
