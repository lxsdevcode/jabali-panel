<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\DnsRecord;
use App\Models\Domain;
use App\Services\Agent\AgentClient;
use App\Services\DomainDeletionService;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Mockery;
use Tests\TestCase;

class DomainDeletionServiceTest extends TestCase
{
    use RefreshDatabase;

    public function test_delete_removes_domain_and_related_records(): void
    {
        $domain = Domain::factory()->create();

        // The DomainObserver auto-creates an SSL record
        $this->assertDatabaseHas('ssl_certificates', ['domain_id' => $domain->id]);

        // Add a DNS record
        DnsRecord::create([
            'domain_id' => $domain->id,
            'type' => 'A',
            'name' => '@',
            'content' => '1.2.3.4',
            'ttl' => 3600,
        ]);

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('send')->zeroOrMoreTimes()->andReturn([]);

        $service = new DomainDeletionService($agent);
        $service->delete($domain);

        $this->assertDatabaseMissing('domains', ['id' => $domain->id]);
        $this->assertDatabaseMissing('ssl_certificates', ['domain_id' => $domain->id]);
        $this->assertDatabaseMissing('dns_records', ['domain_id' => $domain->id]);
    }

    public function test_delete_handles_agent_failure_gracefully(): void
    {
        $domain = Domain::factory()->create();

        $agent = Mockery::mock(AgentClient::class);
        $agent->shouldReceive('send')
            ->andThrow(new \Exception('Agent unreachable'));

        $service = new DomainDeletionService($agent);
        $service->delete($domain);

        // Domain should still be deleted even if agent fails
        $this->assertDatabaseMissing('domains', ['id' => $domain->id]);
    }
}
