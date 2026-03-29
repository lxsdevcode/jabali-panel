<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;
use Illuminate\Database\Eloquent\Relations\HasOne;

class Domain extends Model
{
    use HasFactory;

    protected static function booted(): void
    {
        static::deleting(function (Domain $domain) {
            app(\App\Services\DomainDeletionService::class)->beforeDelete($domain);
        });
    }

    protected $fillable = [
        'user_id',
        'domain',
        'document_root',
        'ip_address',
        'ipv6_address',
        'is_active',
        'ssl_enabled',
        'directory_index',
        'page_cache_enabled',
        'dns_status',
        'dns_resolved_ip',
        'dns_checked_at',
        'whois_status',
        'whois_expiry',
    ];

    protected function casts(): array
    {

        return [
            'is_active' => 'boolean',
            'ssl_enabled' => 'boolean',
            'page_cache_enabled' => 'boolean',
            'dns_checked_at' => 'datetime',
            'whois_expiry' => 'date',
        ];

    }

    public function user(): BelongsTo
    {
        return $this->belongsTo(User::class);
    }

    public function dnsRecords(): HasMany
    {
        return $this->hasMany(DnsRecord::class);
    }

    public function emailDomain(): HasOne
    {
        return $this->hasOne(EmailDomain::class);
    }

    public function sslCertificate(): HasOne
    {
        return $this->hasOne(SslCertificate::class)->where('service', 'web');
    }

    public function mailSslCertificate(): HasOne
    {
        return $this->hasOne(SslCertificate::class)->where('service', 'mail');
    }

    public function sslCertificates(): HasMany
    {
        return $this->hasMany(SslCertificate::class);
    }

    public function bandwidthUsage(): HasMany
    {
        return $this->hasMany(DomainBandwidthUsage::class);
    }

    public function currentMonthBandwidth(): int
    {
        return (int) $this->bandwidthUsage()
            ->whereYear('date', now()->year)
            ->whereMonth('date', now()->month)
            ->sum('bytes_out');
    }

    public function redirects(): HasMany
    {
        return $this->hasMany(DomainRedirect::class);
    }

    public function aliases(): HasMany
    {
        return $this->hasMany(DomainAlias::class);
    }

    public function hotlinkSetting(): HasOne
    {
        return $this->hasOne(DomainHotlinkSetting::class);
    }

    /**
     * Check if SSL is active for this domain
     */
    public function hasSslActive(): bool
    {
        return $this->sslCertificate()->exists() && $this->sslCertificate->isActive();
    }

    /**
     * Get SSL status for display
     */
    public function getSslStatusAttribute(): string
    {
        if (! $this->sslCertificate) {
            return 'No SSL';
        }

        return $this->sslCertificate->status_label;
    }
}
