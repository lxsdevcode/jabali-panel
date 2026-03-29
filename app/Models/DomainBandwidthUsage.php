<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class DomainBandwidthUsage extends Model
{
    public $timestamps = false;

    protected $table = 'domain_bandwidth_usage';

    protected $fillable = [
        'domain_id',
        'date',
        'bytes_out',
        'requests',
    ];

    protected function casts(): array
    {
        return [
            'date' => 'date',
            'bytes_out' => 'integer',
            'requests' => 'integer',
        ];
    }

    public function domain(): BelongsTo
    {
        return $this->belongsTo(Domain::class);
    }

    public function scopeForMonth($query, int $year, int $month)
    {
        return $query->whereYear('date', $year)->whereMonth('date', $month);
    }
}
