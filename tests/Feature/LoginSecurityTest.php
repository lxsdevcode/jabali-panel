<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Filament\Admin\Pages\Auth\Login as AdminLogin;
use App\Filament\Jabali\Pages\Auth\Login as JabaliLogin;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Auth;
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

    // ── Feature 4: 2FA enforcement for admins ──

    public function test_admin_without_2fa_is_redirected_to_setup(): void
    {
        $admin = User::factory()->admin()->create([
            'email' => 'admin@example.com',
            'password' => 'password',
            'two_factor_secret' => null,
            'two_factor_confirmed_at' => null,
        ]);

        Auth::guard('admin')->login($admin);

        // Accessing admin dashboard should redirect to 2FA setup
        $response = $this->get('/jabali-admin');

        $response->assertRedirect('/jabali-admin/two-factor-setup');
    }

    public function test_admin_with_2fa_can_access_dashboard(): void
    {
        $admin = User::factory()->admin()->create([
            'email' => 'admin@example.com',
            'password' => 'password',
            'two_factor_secret' => encrypt('test-secret'),
            'two_factor_confirmed_at' => now(),
        ]);

        Auth::guard('admin')->login($admin);

        // /jabali-admin redirects to /jabali-admin/dashboard — follow the redirect
        $response = $this->get('/jabali-admin');

        // Should NOT redirect to 2FA setup
        $this->assertStringNotContainsString('two-factor-setup', $response->headers->get('Location', ''));
    }

    public function test_regular_user_not_forced_to_setup_2fa(): void
    {
        $user = User::factory()->create([
            'email' => 'user@example.com',
            'password' => 'password',
            'two_factor_secret' => null,
        ]);

        Auth::guard('web')->login($user);

        $response = $this->get('/jabali-panel');

        // Should not redirect to 2FA setup — 2FA is optional for regular users
        $response->assertOk();
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
