<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Filament\Admin\Pages\Backups;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Livewire\Livewire;
use Tests\TestCase;

class TabSkeletonTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_backups_page_contains_skeleton_loader(): void
    {
        $admin = User::factory()->admin()->create();

        Livewire::actingAs($admin, 'admin')
            ->test(Backups::class)
            ->assertSeeHtml('animate-pulse');
    }
}
