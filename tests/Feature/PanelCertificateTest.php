<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\PanelCertificate;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class PanelCertificateTest extends TestCase
{
    use RefreshDatabase;

    public function test_panel_certificate_model_is_active(): void
    {
        $cert = PanelCertificate::factory()->create();

        $this->assertTrue($cert->isActive());
    }

    public function test_panel_certificate_model_detects_expired(): void
    {
        $cert = PanelCertificate::factory()->expired()->create();

        $this->assertTrue($cert->isExpired());
        $this->assertEquals('danger', $cert->status_color);
    }

    public function test_panel_certificate_model_detects_expiring_soon(): void
    {
        $cert = PanelCertificate::factory()->create([
            'expires_at' => now()->addDays(15),
        ]);

        $this->assertTrue($cert->isExpiringSoon(30));
        $this->assertFalse($cert->isExpiringSoon(10));
    }

    public function test_panel_certificate_model_detects_self_signed(): void
    {
        $cert = PanelCertificate::factory()->selfSigned()->create();

        $this->assertTrue($cert->isSelfSigned());
        $this->assertEquals('Self-Signed', $cert->status_label);
    }

    public function test_panel_certificate_days_until_expiry(): void
    {
        $cert = PanelCertificate::factory()->create([
            'expires_at' => now()->addDays(45),
        ]);

        $this->assertContains($cert->days_until_expiry, [44, 45]);
    }

    public function test_panel_cert_sync_command_warns_when_no_hostname(): void
    {
        config(['jabali.panel.hostname' => '']);
        config(['app.url' => 'http://localhost']);

        $this->artisan('jabali:panel-cert-sync')
            ->expectsOutputToContain('No panel hostname')
            ->assertSuccessful();
    }

    public function test_panel_cert_sync_creates_pending_record_when_no_cert_file(): void
    {
        config(['jabali.panel.hostname' => 'panel.example.com']);
        config(['jabali.panel.cert_storage' => '/tmp/nonexistent-jabali-test']);

        $this->artisan('jabali:panel-cert-sync')
            ->assertSuccessful();

        $this->assertDatabaseHas('panel_certificates', [
            'hostname' => 'panel.example.com',
            'status' => 'pending',
        ]);
    }

    public function test_panel_certificate_failed_factory_state(): void
    {
        $cert = PanelCertificate::factory()->failed()->create();

        $this->assertEquals('failed', $cert->status);
        $this->assertEquals('danger', $cert->status_color);
        $this->assertNotNull($cert->last_error);
    }
}
