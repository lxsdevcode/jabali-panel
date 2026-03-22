<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class DomainRedirect extends Model
{
    protected $fillable = [
        'domain_id',
        'source_path',
        'destination_url',
        'redirect_type',
        'is_wildcard',
        'is_active',
    ];

    protected function casts(): array
    {

        return [
            'is_wildcard' => 'boolean',
            'is_active' => 'boolean',
        ];

    }

    public function domain(): BelongsTo
    {
        return $this->belongsTo(Domain::class);
    }
}
