<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Services\Dns\PowerDnsService;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class PowerDnsServiceTest extends TestCase
{
    protected function setUp(): void
    {
        parent::setUp();

        config([
            'powerdns.api_url' => 'http://127.0.0.1:8081',
            'powerdns.api_key' => 'test-api-key',
            'powerdns.server_id' => 'localhost',
        ]);
    }

    public function test_create_zone_sends_correct_api_request(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones' => Http::response(['id' => 'example.com.'], 201),
        ]);

        $service = new PowerDnsService;
        $service->createZone('example.com', ['ns1.example.com', 'ns2.example.com'], '1.2.3.4');

        Http::assertSent(function ($request) {
            return $request->url() === 'http://127.0.0.1:8081/api/v1/servers/localhost/zones'
                && $request['name'] === 'example.com.'
                && $request['kind'] === 'Native'
                && $request['nameservers'] === ['ns1.example.com.', 'ns2.example.com.']
                && $request->hasHeader('X-API-Key', 'test-api-key');
        });
    }

    public function test_delete_zone_sends_delete_request(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones/example.com.' => Http::response(null, 204),
        ]);

        $service = new PowerDnsService;
        $service->deleteZone('example.com');

        Http::assertSent(fn ($r) => $r->method() === 'DELETE');
    }

    public function test_delete_zone_ignores_404(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones/example.com.' => Http::response(null, 404),
        ]);

        $service = new PowerDnsService;
        $service->deleteZone('example.com'); // Should not throw

        $this->assertTrue(true);
    }

    public function test_set_records_groups_by_rrset(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones/example.com.' => Http::response(null, 204),
        ]);

        $service = new PowerDnsService;
        $service->setRecords('example.com', [
            ['name' => '@', 'type' => 'A', 'content' => '1.2.3.4', 'ttl' => 3600],
            ['name' => 'www', 'type' => 'A', 'content' => '1.2.3.4', 'ttl' => 3600],
            ['name' => '@', 'type' => 'MX', 'content' => 'mail.example.com', 'ttl' => 3600, 'priority' => 10],
        ]);

        Http::assertSent(function ($request) {
            $rrsets = $request['rrsets'];

            return count($rrsets) === 3
                && $request->method() === 'PATCH';
        });
    }

    public function test_enable_dnssec_creates_csk(): void
    {
        Http::fake([
            '*/cryptokeys' => Http::response([
                'id' => 1,
                'keytype' => 'csk',
                'ds' => ['12345 13 2 abc123'],
            ], 201),
        ]);

        $service = new PowerDnsService;
        $result = $service->enableDnssec('example.com');

        $this->assertArrayHasKey('ds', $result);

        Http::assertSent(function ($request) {
            return $request->method() === 'POST'
                && $request['keytype'] === 'csk'
                && $request['active'] === true;
        });
    }

    public function test_get_ds_records_parses_response(): void
    {
        Http::fake([
            '*/cryptokeys' => Http::response([
                ['id' => 1, 'ds' => ['12345 13 2 abcdef0123456789']],
            ], 200),
        ]);

        $service = new PowerDnsService;
        $ds = $service->getDsRecords('example.com');

        $this->assertCount(1, $ds);
        $this->assertEquals(12345, $ds[0]['tag']);
        $this->assertEquals(13, $ds[0]['algorithm']);
        $this->assertEquals(2, $ds[0]['digest_type']);
        $this->assertEquals('abcdef0123456789', $ds[0]['digest']);
    }

    public function test_zone_exists_returns_true_for_existing_zone(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones/example.com.' => Http::response(['id' => 'example.com.'], 200),
        ]);

        $service = new PowerDnsService;
        $this->assertTrue($service->zoneExists('example.com'));
    }

    public function test_zone_exists_returns_false_for_missing_zone(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones/missing.com.' => Http::response(null, 404),
        ]);

        $service = new PowerDnsService;
        $this->assertFalse($service->zoneExists('missing.com'));
    }

    public function test_create_zone_throws_on_api_error(): void
    {
        Http::fake([
            '*/api/v1/servers/localhost/zones' => Http::response(['error' => 'Already exists'], 422),
        ]);

        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('Already exists');

        $service = new PowerDnsService;
        $service->createZone('example.com', ['ns1.example.com'], '1.2.3.4');
    }
}
