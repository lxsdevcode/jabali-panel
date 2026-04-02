<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Filament\Admin\Pages\ServerSettings;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Livewire\Livewire;
use Tests\TestCase;

class ServerSettingsBrandingTest extends TestCase
{
    use RefreshDatabase;

    private User $admin;

    protected function setUp(): void
    {
        parent::setUp();
        $this->admin = User::factory()->admin()->create();
    }

    public function test_server_settings_page_loads(): void
    {
        $response = $this->actingAs($this->admin, 'admin')
            ->get('/jabali-admin/server-settings');

        $response->assertOk();
    }

    public function test_save_branding_method_is_callable(): void
    {
        Livewire::actingAs($this->admin, 'admin')
            ->test(ServerSettings::class)
            ->set('brandingData.panel_name', 'Test Panel')
            ->call('saveBranding')
            ->assertNotified();
    }

    public function test_branding_buttons_have_same_structure_as_hostname(): void
    {
        $html = Livewire::actingAs($this->admin, 'admin')
            ->test(ServerSettings::class)
            ->html();

        $dom = new \DOMDocument;
        @$dom->loadHTML($html);
        $xpath = new \DOMXPath($dom);

        // Check Save Branding button
        $brandingButtons = $xpath->query('//button[contains(., "Save Branding")]');
        $this->assertGreaterThan(0, $brandingButtons->length, 'Save Branding button not found');

        $brandingBtn = $brandingButtons->item(0);
        $brandingWireClick = $brandingBtn->getAttribute('wire:click');
        $this->assertEquals('saveBranding', $brandingWireClick);

        // Check Save Hostname button
        $hostnameButtons = $xpath->query('//button[contains(., "Save Hostname")]');
        $this->assertGreaterThan(0, $hostnameButtons->length, 'Save Hostname button not found');

        $hostnameBtn = $hostnameButtons->item(0);
        $hostnameWireClick = $hostnameBtn->getAttribute('wire:click');
        $this->assertEquals('saveHostname', $hostnameWireClick);

        // Verify both have the same parent structure (fi-sc-actions > fi-ac)
        $brandingParent1 = $brandingBtn->parentNode;
        $brandingParent2 = $brandingParent1?->parentNode;
        $hostnameParent1 = $hostnameBtn->parentNode;
        $hostnameParent2 = $hostnameParent1?->parentNode;

        $brandingP2Class = $brandingParent2 instanceof \DOMElement ? $brandingParent2->getAttribute('class') : '';
        $hostnameP2Class = $hostnameParent2 instanceof \DOMElement ? $hostnameParent2->getAttribute('class') : '';

        // Both should be inside fi-sc-actions (Actions::make layout)
        fwrite(STDERR, "\n[DEBUG] Save Branding parent classes: ".
            ($brandingParent1 instanceof \DOMElement ? $brandingParent1->getAttribute('class') : 'n/a').
            ' > '.$brandingP2Class."\n");
        fwrite(STDERR, '[DEBUG] Save Hostname parent classes: '.
            ($hostnameParent1 instanceof \DOMElement ? $hostnameParent1->getAttribute('class') : 'n/a').
            ' > '.$hostnameP2Class."\n");

        $this->assertStringContainsString('fi-sc-actions', $brandingP2Class,
            'Save Branding should be inside fi-sc-actions wrapper (same as hostname)');
    }
}
