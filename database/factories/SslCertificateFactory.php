<?php

declare(strict_types=1);

namespace Database\Factories;

use App\Models\Domain;
use App\Models\SslCertificate;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends Factory<SslCertificate>
 */
class SslCertificateFactory extends Factory
{
    public function definition(): array
    {
        return [
            'domain_id' => Domain::factory(),
            'service' => 'web',
            'type' => 'lets_encrypt',
            'status' => 'active',
            'issuer' => "Let's Encrypt",
            'issued_at' => now()->subDays(30),
            'expires_at' => now()->addDays(60),
            'last_check_at' => now(),
            'auto_renew' => true,
            'renewal_attempts' => 0,
        ];
    }
}
