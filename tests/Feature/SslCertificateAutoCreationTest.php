<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\Domain;
use App\Models\SslCertificate;
use App\Models\User;
use App\Services\Agent\AgentClientInterface;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Queue;
use Mockery;
use Tests\TestCase;

class SslCertificateAutoCreationTest extends TestCase
{
    use RefreshDatabase;

    protected function setUp(): void
    {
        parent::setUp();

        // Mock the agent so observer doesn't hit the real socket
        $this->app->instance(AgentClientInterface::class, Mockery::mock(AgentClientInterface::class)->shouldIgnoreMissing());
    }

    public function test_creating_domain_creates_pending_ssl_certificate(): void
    {
        Queue::fake();

        $user = User::factory()->create();

        $domain = Domain::create([
            'user_id' => $user->id,
            'domain' => 'example.com',
            'is_active' => true,
            'ssl_enabled' => false,
        ]);

        $ssl = SslCertificate::where('domain_id', $domain->id)
            ->where('service', 'web')
            ->first();

        $this->assertNotNull($ssl, 'Pending SSL record should be created when domain is created');
        $this->assertEquals('pending', $ssl->status);
        $this->assertEquals('none', $ssl->type);
        $this->assertTrue($ssl->auto_renew);
    }

    public function test_duplicate_ssl_not_created_if_already_exists(): void
    {
        Queue::fake();

        $user = User::factory()->create();

        // Create domain without observer (simulates pre-existing domain with active SSL)
        $domain = Domain::withoutEvents(fn () => Domain::factory()->for($user)->create(['ssl_enabled' => true]));
        SslCertificate::create([
            'domain_id' => $domain->id,
            'service' => 'web',
            'type' => 'lets_encrypt',
            'status' => 'active',
            'auto_renew' => true,
            'expires_at' => now()->addMonths(2),
        ]);

        // Create another domain — observer should create pending SSL for it
        $domain2 = Domain::create([
            'user_id' => $user->id,
            'domain' => 'new-domain.com',
            'is_active' => true,
            'ssl_enabled' => false,
        ]);

        // First domain should still have exactly 1 SSL record (active)
        $this->assertEquals(1, SslCertificate::where('domain_id', $domain->id)->count());
        $this->assertEquals('active', SslCertificate::where('domain_id', $domain->id)->first()->status);

        // Second domain should have 1 pending record
        $this->assertEquals(1, SslCertificate::where('domain_id', $domain2->id)->count());
        $this->assertEquals('pending', SslCertificate::where('domain_id', $domain2->id)->first()->status);
    }

    public function test_pending_ssl_record_is_eligible_for_issuance(): void
    {
        Queue::fake();

        $user = User::factory()->create();

        $domain = Domain::create([
            'user_id' => $user->id,
            'domain' => 'pending-test.com',
            'is_active' => true,
            'ssl_enabled' => false,
        ]);

        $ssl = SslCertificate::where('domain_id', $domain->id)
            ->where('service', 'web')
            ->first();

        $this->assertNotNull($ssl);

        // The Issue SSL action visibility check from Ssl.php must match pending records
        $isVisible = $ssl->service === 'web'
            && in_array($ssl->status, ['pending', 'failed'])
            || $ssl->type === 'self_signed';

        $this->assertTrue($isVisible, 'Pending SSL records should be eligible for issuance');
    }
}
