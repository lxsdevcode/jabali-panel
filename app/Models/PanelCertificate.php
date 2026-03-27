<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

class PanelCertificate extends Model
{
    /** @use HasFactory<\Database\Factories\PanelCertificateFactory> */
    use HasFactory;

    protected $fillable = [
        'hostname',
        'status',
        'issuer',
        'issued_at',
        'expires_at',
        'last_renewal_at',
        'last_renewal_result',
        'last_error',
        'serial_number',
    ];

    protected function casts(): array
    {
        return [
            'issued_at' => 'datetime',
            'expires_at' => 'datetime',
            'last_renewal_at' => 'datetime',
        ];
    }

    public function isActive(): bool
    {
        return $this->status === 'active' && $this->expires_at && $this->expires_at->isFuture();
    }

    public function isExpired(): bool
    {
        return $this->expires_at && $this->expires_at->isPast();
    }

    public function isExpiringSoon(int $days = 30): bool
    {
        if (! $this->expires_at) {
            return false;
        }

        $daysUntilExpiry = now()->diffInDays($this->expires_at, false);

        return $daysUntilExpiry >= 0 && $daysUntilExpiry <= $days;
    }

    public function getDaysUntilExpiryAttribute(): ?int
    {
        if (! $this->expires_at) {
            return null;
        }

        return (int) now()->diffInDays($this->expires_at, false);
    }

    public function getStatusColorAttribute(): string
    {
        if ($this->isExpired() || $this->isExpiringSoon(7)) {
            return 'danger';
        }

        if ($this->isExpiringSoon(30)) {
            return 'warning';
        }

        return match ($this->status) {
            'active' => 'success',
            'pending' => 'info',
            'failed' => 'danger',
            default => 'gray',
        };
    }

    public function getStatusLabelAttribute(): string
    {
        if ($this->isExpired()) {
            return 'Expired';
        }

        if ($this->status === 'active' && $this->isExpiringSoon(7)) {
            return 'Expiring Soon';
        }

        return match ($this->status) {
            'active' => 'Active',
            'pending' => 'Pending',
            'failed' => 'Failed',
            'self_signed' => 'Self-Signed',
            default => 'Unknown',
        };
    }

    public function isSelfSigned(): bool
    {
        return $this->status === 'self_signed'
            || str_contains((string) $this->issuer, 'Caddy Local Authority');
    }
}
