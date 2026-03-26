<?php

declare(strict_types=1);

namespace Database\Factories;

use App\Models\PanelCertificate;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends Factory<PanelCertificate>
 */
class PanelCertificateFactory extends Factory
{
    public function definition(): array
    {
        return [
            'hostname' => fake()->domainName(),
            'status' => 'active',
            'issuer' => "Let's Encrypt",
            'issued_at' => now()->subDays(30),
            'expires_at' => now()->addDays(60),
            'last_renewal_at' => now()->subDays(30),
            'last_renewal_result' => 'success',
            'serial_number' => fake()->sha1(),
        ];
    }

    public function selfSigned(): static
    {
        return $this->state(fn () => [
            'status' => 'self_signed',
            'issuer' => 'Caddy Local Authority',
        ]);
    }

    public function expired(): static
    {
        return $this->state(fn () => [
            'status' => 'active',
            'expires_at' => now()->subDays(5),
        ]);
    }

    public function failed(): static
    {
        return $this->state(fn () => [
            'status' => 'failed',
            'last_renewal_result' => 'failed',
            'last_error' => 'ACME challenge failed: DNS not resolving',
        ]);
    }
}
