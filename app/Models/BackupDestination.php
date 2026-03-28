<?php

declare(strict_types=1);

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class BackupDestination extends Model
{
    use HasFactory;

    protected $fillable = [
        'user_id',
        'name',
        'type',
        'config',
        'is_default',
        'is_active',
        'is_server_backup',
        'last_tested_at',
        'test_status',
        'test_message',
    ];

    protected function casts(): array
    {
        return [
            'config' => 'encrypted:array',
            'is_default' => 'boolean',
            'is_active' => 'boolean',
            'is_server_backup' => 'boolean',
            'last_tested_at' => 'datetime',
        ];
    }

    public function user(): BelongsTo
    {
        return $this->belongsTo(User::class);
    }

    public function backups(): HasMany
    {
        return $this->hasMany(Backup::class, 'destination_id');
    }

    public function schedules(): HasMany
    {
        return $this->hasMany(BackupSchedule::class, 'destination_id');
    }

    /**
     * Check if destination is local storage.
     */
    public function isLocal(): bool
    {
        return $this->type === 'local';
    }

    /**
     * Check if destination is remote storage.
     */
    public function isRemote(): bool
    {
        return in_array($this->type, ['sftp', 'nfs', 's3', 'b2', 'wasabi', 'minio', 'gcs', 'azure', 'rest']);
    }

    /**
     * Get the display label for the destination type.
     */
    public function getTypeLabelAttribute(): string
    {
        return match ($this->type) {
            'local' => 'Local Storage',
            'sftp' => 'SFTP Server',
            'nfs' => 'NFS Mount',
            's3' => 'S3-Compatible Storage',
            default => ucfirst($this->type),
        };
    }

    /**
     * Scope for active destinations.
     */
    public function scopeActive($query)
    {
        return $query->where('is_active', true);
    }

    /**
     * Scope for user destinations.
     */
    public function scopeForUser($query, int $userId)
    {
        return $query->where('user_id', $userId);
    }

    /**
     * Scope for server backup destinations (admin-level).
     */
    public function scopeServerBackups($query)
    {
        return $query->where('is_server_backup', true);
    }

    /**
     * Get the Restic repository URL for this destination.
     */
    public function getResticRepoUrl(): string
    {
        $config = $this->config ?? [];

        return match ($this->type) {
            'local' => $config['path'] ?? '/var/backups/jabali/restic',
            'sftp' => sprintf('sftp:%s@%s:%s',
                $config['username'] ?? 'backup',
                $config['host'] ?? 'localhost',
                $config['path'] ?? '/backups',
            ),
            's3', 'b2', 'wasabi', 'minio' => sprintf('s3:%s/%s',
                $config['endpoint'] ?? 'https://s3.amazonaws.com',
                $config['bucket'] ?? 'jabali-backups',
            ),
            'gcs' => sprintf('gs:%s/', $config['bucket'] ?? 'jabali-backups'),
            'azure' => sprintf('azure:%s:/', $config['container'] ?? 'jabali-backups'),
            'rest' => sprintf('rest:%s', rtrim($config['url'] ?? 'http://localhost:8000', '/')),
            default => '/var/backups/jabali/restic',
        };
    }

    /**
     * Get environment variables needed for Restic auth.
     *
     * @return array<string, string>
     */
    public function getResticEnv(): array
    {
        $config = $this->config ?? [];
        $env = ['RESTIC_PASSWORD_FILE' => '/etc/jabali/restic-password'];

        if ($this->type === 'sftp' && ! empty($config['password'])) {
            $env['SSHPASS'] = $config['password'];
        }

        // S3-compatible (S3, B2, Wasabi, MinIO)
        if (in_array($this->type, ['s3', 'b2', 'wasabi', 'minio'])) {
            if (! empty($config['access_key'])) {
                $env['AWS_ACCESS_KEY_ID'] = $config['access_key'];
            }
            if (! empty($config['secret_key'])) {
                $env['AWS_SECRET_ACCESS_KEY'] = $config['secret_key'];
            }
        }

        // Google Cloud Storage
        if ($this->type === 'gcs') {
            if (! empty($config['access_key'])) {
                $env['GOOGLE_PROJECT_ID'] = $config['access_key'];
            }
            if (! empty($config['secret_key'])) {
                $env['GOOGLE_APPLICATION_CREDENTIALS'] = $config['secret_key'];
            }
        }

        // Azure Blob Storage
        if ($this->type === 'azure') {
            if (! empty($config['account'])) {
                $env['AZURE_ACCOUNT_NAME'] = $config['account'];
            }
            if (! empty($config['key'])) {
                $env['AZURE_ACCOUNT_KEY'] = $config['key'];
            }
        }

        return $env;
    }
}
