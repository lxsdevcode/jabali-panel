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

    public function test_branding_buttons_have_wire_click_directives(): void
    {
        $livewire = Livewire::actingAs($this->admin, 'admin')
            ->test(ServerSettings::class);

        // First, upload a test logo so the Remove button appears
        $livewire->set('currentLogo', 'test-logo.png');

        $html = $livewire->html();

        $dom = new \DOMDocument;
        @$dom->loadHTML($html);
        $xpath = new \DOMXPath($dom);

        // Check Save Branding button has wire:click directive
        $brandingButtons = $xpath->query('//button[contains(., "Save Branding")]');
        $this->assertGreaterThan(0, $brandingButtons->length, 'Save Branding button not found');

        $brandingBtn = $brandingButtons->item(0);
        $brandingWireClick = $brandingBtn->getAttribute('wire:click');
        $this->assertEquals('saveBranding', $brandingWireClick);

        // Check Upload Light Logo button has wire:click directive
        $uploadLightButtons = $xpath->query('//button[contains(., "Upload Light Logo")]');
        $this->assertGreaterThan(0, $uploadLightButtons->length, 'Upload Light Logo button not found');

        $uploadLightBtn = $uploadLightButtons->item(0);
        $uploadLightWireClick = $uploadLightBtn->getAttribute('wire:click');
        $this->assertEquals('openUploadLogoLight', $uploadLightWireClick);

        // Check Upload Dark Logo button has wire:click directive
        $uploadDarkButtons = $xpath->query('//button[contains(., "Upload Dark Logo")]');
        $this->assertGreaterThan(0, $uploadDarkButtons->length, 'Upload Dark Logo button not found');

        $uploadDarkBtn = $uploadDarkButtons->item(0);
        $uploadDarkWireClick = $uploadDarkBtn->getAttribute('wire:click');
        $this->assertEquals('openUploadLogoDark', $uploadDarkWireClick);

        // Check Remove Logos button has wire:click directive
        $removeButtons = $xpath->query('//button[contains(., "Remove Logos")]');
        $this->assertGreaterThan(0, $removeButtons->length, 'Remove Logos button not found');

        $removeBtn = $removeButtons->item(0);
        $removeWireClick = $removeBtn->getAttribute('wire:click');
        $this->assertEquals('removeLogo', $removeWireClick);
    }
}
