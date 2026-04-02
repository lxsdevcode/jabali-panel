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

    public function test_branding_section_renders_with_file_inputs(): void
    {
        $html = Livewire::actingAs($this->admin, 'admin')
            ->test(ServerSettings::class)
            ->html();

        $this->assertStringContainsString('wire:model="logoLightUpload"', $html);
        $this->assertStringContainsString('wire:model="logoDarkUpload"', $html);
        $this->assertStringContainsString('Save Branding', $html);
        $this->assertStringContainsString('Save Light Logo', $html);
        $this->assertStringContainsString('Save Dark Logo', $html);
    }
}
