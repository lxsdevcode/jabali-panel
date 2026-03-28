<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class TabSkeletonTest extends TestCase
{
    use RefreshDatabase;

    public function test_admin_page_includes_skeleton_script(): void
    {
        $admin = User::factory()->admin()->create();

        $response = $this->actingAs($admin, 'admin')
            ->get('/jabali-admin/backups');

        $response->assertOk();
    }
}
