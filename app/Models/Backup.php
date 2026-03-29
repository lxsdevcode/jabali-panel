<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Backup extends Model
{
    use HasFactory;

    protected $fillable = [
        'user_id',
        'destination_id',
        'schedule_id',
        'name',
        'filename',
        'type',
        'include_files',
        'include_databases',
        'include_mailboxes',
        'include_dns',
        'include_ssl',
        'domains',
        'databases',
        'mailboxes',
        'users',
        'size_bytes',
        'file_count',
        'status',
        'local_path',
        'remote_path',
        'snapshot_id',
        'checksum',
        'started_at',
        'completed_at',
        'error_message',
        'metadata',
        'expires_at',
    ];

    protected function casts(): array
    {
        return [
            'include_files' => 'boolean',
            'include_databases' => 'boolean',
            'include_mailboxes' => 'boolean',
            'include_dns' => 'boolean',
            'include_ssl' => 'boolean',
            'domains' => 'array',
            'databases' => 'array',
            'mailboxes' => 'array',
            'users' => 'array',
            'size_bytes' => 'integer',
            'file_count' => 'integer',
            'metadata' => 'array',
            'started_at' => 'datetime',
            'completed_at' => 'datetime',
            'expires_at' => 'datetime',
        ];
    }

    public function user(): BelongsTo
    {
        return $this->belongsTo(User::class);
    }

    public function destination(): BelongsTo
    {
        return $this->belongsTo(BackupDestination::class, 'destination_id');
    }

    public function schedule(): BelongsTo
    {
        return $this->belongsTo(BackupSchedule::class, 'schedule_id');
    }

    public function restores(): HasMany
    {
        return $this->hasMany(BackupRestore::class);
    }

    /**
     * Get human-readable file size.
     */
    public function getSizeHumanAttribute(): string
    {
        $bytes = $this->size_bytes;

        if ($bytes >= 1073741824) {
            return number_format($bytes / 1073741824, 2).' GB';
        } elseif ($bytes >= 1048576) {
            return number_format($bytes / 1048576, 2).' MB';
        } elseif ($bytes >= 1024) {
            return number_format($bytes / 1024, 2).' KB';
        }

        return $bytes.' bytes';
    }

    /**
     * Get backup duration in human-readable format.
     */
    public function getDurationAttribute(): ?string
    {
        if (! $this->started_at || ! $this->completed_at) {
            return null;
        }

        $seconds = $this->completed_at->diffInSeconds($this->started_at);

        if ($seconds >= 3600) {
            return gmdate('H:i:s', $seconds);
        } elseif ($seconds >= 60) {
            return gmdate('i:s', $seconds).' min';
        }

        return $seconds.' sec';
    }

    /**
     * Check if backup has expired.
     */
    public function getIsExpiredAttribute(): bool
    {
        return $this->expires_at && $this->expires_at->isPast();
    }

    /**
     * Check if backup is stored locally.
     */
    public function isLocal(): bool
    {
        return $this->destination_id === null || $this->destination?->isLocal();
    }

    /**
     * Check if backup is stored remotely.
     */
    public function isRemote(): bool
    {
        return $this->destination && $this->destination->isRemote();
    }

    /**
     * Check if backup can be downloaded directly.
     */
    public function canDownload(): bool
    {
        return $this->status === 'completed' && $this->local_path && file_exists($this->local_path);
    }

    /**
     * Get status badge color for UI.
     */
    public function getStatusColorAttribute(): string
    {
        return match ($this->status) {
            'completed' => 'success',
            'failed' => 'danger',
            'running', 'uploading' => 'warning',
            'pending' => 'gray',
            default => 'gray',
        };
    }

    /**
     * Get status label for UI.
     */
    public function getStatusLabelAttribute(): string
    {
        return match ($this->status) {
            'pending' => 'Pending',
            'running' => 'Creating...',
            'uploading' => 'Uploading...',
            'completed' => 'Completed',
            'failed' => 'Failed',
            default => ucfirst($this->status),
        };
    }
}
