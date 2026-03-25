<?php

declare(strict_types=1);

namespace Tests\Feature;

use App\Models\ImpersonationToken;
use App\Models\User;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ImpersonationSecurityTest extends TestCase
{
    use RefreshDatabase;

    public function test_impersonation_token_rejects_different_ip_even_behind_proxy(): void
    {
        $admin = User::factory()->admin()->create();
        $target = User::factory()->create();

        // Create token from IP 1.2.3.4
        $token = ImpersonationToken::createForUser($admin, $target, '1.2.3.4');

        // Simulate being behind a reverse proxy (production scenario)
        request()->setTrustedProxies(['127.0.0.1'], \Illuminate\Http\Request::HEADER_X_FORWARDED_FOR);

        // Try to use token from different IP — should STILL fail
        $result = ImpersonationToken::findValidToken($token->token, '5.6.7.8');

        $this->assertNull($result, 'Token should be rejected when IP does not match, even behind proxy');

        // Reset trusted proxies
        request()->setTrustedProxies([], \Illuminate\Http\Request::HEADER_X_FORWARDED_FOR);
    }

    public function test_impersonation_token_accepts_same_ip(): void
    {
        $admin = User::factory()->admin()->create();
        $target = User::factory()->create();

        $token = ImpersonationToken::createForUser($admin, $target, '1.2.3.4');

        $result = ImpersonationToken::findValidToken($token->token, '1.2.3.4');

        $this->assertNotNull($result, 'Token should be accepted when IP matches');
    }

    public function test_impersonation_token_rejects_expired_token(): void
    {
        $admin = User::factory()->admin()->create();
        $target = User::factory()->create();

        $token = ImpersonationToken::createForUser($admin, $target, '1.2.3.4');

        $this->travel(6)->minutes();

        $result = ImpersonationToken::findValidToken($token->token, '1.2.3.4');

        $this->assertNull($result, 'Expired token should be rejected');
    }

    public function test_impersonation_token_rejects_already_used_token(): void
    {
        $admin = User::factory()->admin()->create();
        $target = User::factory()->create();

        $token = ImpersonationToken::createForUser($admin, $target, '1.2.3.4');
        $token->markAsUsed();

        $result = ImpersonationToken::findValidToken($token->token, '1.2.3.4');

        $this->assertNull($result, 'Used token should be rejected');
    }
}
