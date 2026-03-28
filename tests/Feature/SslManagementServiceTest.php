<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\Domain;
use App\Models\SslCertificate;
use App\Services\Agent\AgentClient;
use App\Services\SslManagementService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Mockery;
use Tests\TestCase;

class SslManagementServiceTest extends TestCase
{
    use RefreshDatabase;

    public function test_issue_creates_active_certificate_record(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslIssue')
            ->once()
            ->with($domain->domain, $domain->user->username, Mockery::any(), true)
            ->andReturn([
                'success' => true,
                'certificate' => 'fake-cert-pem',
                'valid_to' => now()->addDays(90)->toIso8601String(),
            ]);

        $service = new SslManagementService($agent);
        $cert = $service->issue($domain);

        $this->assertInstanceOf(SslCertificate::class, $cert);
        $this->assertEquals('active', $cert->status);
        $this->assertEquals('lets_encrypt', $cert->type);
        $this->assertEquals("Let's Encrypt", $cert->issuer);
        $this->assertEquals($domain->id, $cert->domain_id);
        $this->assertEquals('web', $cert->service);
        $this->assertTrue($cert->auto_renew);
    }

    public function test_issue_stores_failure_when_agent_throws(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslIssue')
            ->once()
            ->andThrow(new \Exception('ACME challenge failed'));

        $service = new SslManagementService($agent);
        $cert = $service->issue($domain);

        $this->assertEquals('failed', $cert->status);
        $this->assertEquals('ACME challenge failed', $cert->last_error);
        $this->assertEquals($domain->id, $cert->domain_id);
    }

    public function test_renew_updates_existing_certificate(): void
    {
        $domain = Domain::factory()->create();
        // DomainObserver auto-creates a pending SSL record — update it to simulate an existing cert
        $existing = SslCertificate::where('domain_id', $domain->id)->where('service', 'web')->first();
        $existing->update([
            'type' => 'lets_encrypt',
            'status' => 'active',
            'issuer' => "Let's Encrypt",
            'expires_at' => now()->addDays(10),
            'renewal_attempts' => 2,
        ]);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslRenew')
            ->once()
            ->with($domain->domain, $domain->user->username)
            ->andReturn([
                'success' => true,
                'valid_to' => now()->addDays(90)->toIso8601String(),
            ]);

        $service = new SslManagementService($agent);
        $cert = $service->renew($domain);

        $this->assertEquals($existing->id, $cert->id);
        $this->assertEquals('active', $cert->status);
        $this->assertEquals(0, $cert->renewal_attempts);
        $this->assertTrue($cert->expires_at->greaterThan(now()->addDays(80)));
    }

    public function test_renew_handles_agent_failure(): void
    {
        $domain = Domain::factory()->create();
        $existing = SslCertificate::where('domain_id', $domain->id)->where('service', 'web')->first();
        $existing->update(['type' => 'lets_encrypt', 'status' => 'active', 'renewal_attempts' => 1]);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslRenew')
            ->once()
            ->andThrow(new \Exception('Renewal failed'));

        $service = new SslManagementService($agent);
        $cert = $service->renew($domain);

        $this->assertEquals('failed', $cert->status);
        $this->assertEquals('Renewal failed', $cert->last_error);
        $this->assertEquals(2, $cert->renewal_attempts);
    }

    public function test_check_updates_certificate_from_agent_response(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslCheck')
            ->once()
            ->with($domain->domain, $domain->user->username)
            ->andReturn([
                'has_ssl' => true,
                'issuer' => 'R3',
                'valid_from' => now()->subDays(30)->toIso8601String(),
                'valid_to' => now()->addDays(60)->toIso8601String(),
            ]);

        $service = new SslManagementService($agent);
        $cert = $service->check($domain);

        $this->assertEquals('active', $cert->status);
        $this->assertEquals('R3', $cert->issuer);
        $this->assertNotNull($cert->expires_at);
    }

    public function test_check_sets_pending_when_no_ssl_found(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslCheck')
            ->once()
            ->andReturn(['has_ssl' => false]);

        $service = new SslManagementService($agent);
        $cert = $service->check($domain);

        $this->assertEquals('pending', $cert->status);
    }

    public function test_install_custom_stores_certificate_data(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslInstall')
            ->once()
            ->andReturn(['success' => true]);

        $service = new SslManagementService($agent);
        $cert = $service->installCustom($domain, 'cert-pem', 'key-pem', 'ca-pem');

        $this->assertEquals('active', $cert->status);
        $this->assertEquals('custom', $cert->type);
        $this->assertEquals('cert-pem', $cert->certificate);
        $this->assertFalse($cert->auto_renew);
    }

    public function test_generate_self_signed_creates_record(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('sslGenerateSelfSigned')
            ->once()
            ->with($domain->domain, $domain->user->username, 365)
            ->andReturn(['success' => true]);

        $service = new SslManagementService($agent);
        $cert = $service->generateSelfSigned($domain);

        $this->assertEquals('active', $cert->status);
        $this->assertEquals('self_signed', $cert->type);
        $this->assertFalse($cert->auto_renew);
    }
}
