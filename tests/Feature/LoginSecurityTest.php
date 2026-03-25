<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Filament\Admin\Pages\Auth\Login as AdminLogin;
use App\Filament\Jabali\Pages\Auth\Login as JabaliLogin;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Livewire\Livewire;
use Tests\TestCase;

class LoginSecurityTest extends TestCase
{
    use RefreshDatabase;

    // ── Feature 1: User enumeration prevention ──

    public function test_user_login_rejects_nonexistent_email(): void
    {
        Livewire::test(JabaliLogin::class)
            ->set('data.email', 'nonexistent@example.com')
            ->set('data.password', 'wrongpassword')
            ->call('authenticate')
            ->assertHasFormErrors(['email']);
    }

    public function test_user_login_rejects_wrong_password_for_existing_user(): void
    {
        User::factory()->create([
            'email' => 'user@example.com',
            'password' => 'correctpassword',
        ]);

        Livewire::test(JabaliLogin::class)
            ->set('data.email', 'user@example.com')
            ->set('data.password', 'wrongpassword')
            ->call('authenticate')
            ->assertHasFormErrors(['email']);
    }

    public function test_user_login_succeeds_with_correct_credentials(): void
    {
        User::factory()->create([
            'email' => 'user@example.com',
            'password' => 'correctpassword',
            'is_admin' => false,
        ]);

        Livewire::test(JabaliLogin::class)
            ->set('data.email', 'user@example.com')
            ->set('data.password', 'correctpassword')
            ->call('authenticate')
            ->assertHasNoFormErrors()
            ->assertRedirect();
    }

    // ── Feature 2: Account lockout ──

    public function test_account_locks_after_too_many_failed_attempts(): void
    {
        $user = User::factory()->create([
            'email' => 'victim@example.com',
            'password' => 'correctpassword',
        ]);

        // Simulate 10 failed login attempts, advancing time to avoid per-minute rate limit
        for ($i = 0; $i < 10; $i++) {
            if ($i > 0 && $i % 4 === 0) {
                $this->travel(61)->seconds();
            }

            Livewire::test(JabaliLogin::class)
                ->set('data.email', 'victim@example.com')
                ->set('data.password', 'wrongpassword')
                ->call('authenticate');
        }

        // The 11th attempt with correct password should fail due to lockout
        $this->travel(61)->seconds();

        Livewire::test(JabaliLogin::class)
            ->set('data.email', 'victim@example.com')
            ->set('data.password', 'correctpassword')
            ->call('authenticate')
            ->assertHasFormErrors(['email']);
    }

    public function test_admin_login_locks_after_too_many_failed_attempts(): void
    {
        User::factory()->admin()->create([
            'email' => 'admin@example.com',
            'password' => 'correctpassword',
        ]);

        for ($i = 0; $i < 10; $i++) {
            Livewire::test(AdminLogin::class)
                ->set('data.email', 'admin@example.com')
                ->set('data.password', 'wrongpassword')
                ->call('authenticate');
        }

        // Correct password should fail due to lockout
        Livewire::test(AdminLogin::class)
            ->set('data.email', 'admin@example.com')
            ->set('data.password', 'correctpassword')
            ->call('authenticate')
            ->assertHasFormErrors(['email']);
    }

    public function test_account_lockout_expires_after_cooldown(): void
    {
        $user = User::factory()->create([
            'email' => 'victim@example.com',
            'password' => 'correctpassword',
        ]);

        // Simulate 10 failed attempts, advancing time to avoid per-minute rate limit
        for ($i = 0; $i < 10; $i++) {
            if ($i > 0 && $i % 4 === 0) {
                $this->travel(61)->seconds();
            }

            Livewire::test(JabaliLogin::class)
                ->set('data.email', 'victim@example.com')
                ->set('data.password', 'wrongpassword')
                ->call('authenticate');
        }

        // Travel 16 minutes into the future (lockout is 15 min)
        $this->travel(16)->minutes();

        // Should succeed now
        Livewire::test(JabaliLogin::class)
            ->set('data.email', 'victim@example.com')
            ->set('data.password', 'correctpassword')
            ->call('authenticate')
            ->assertHasNoFormErrors()
            ->assertRedirect();
    }
}
