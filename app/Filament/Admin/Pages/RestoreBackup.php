<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Models\Backup;
use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use App\Services\FileBrowser\BackupSnapshotAdapter;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Illuminate\Contracts\Support\Htmlable;

class RestoreBackup extends Page implements HasActions, HasForms
{
    use InteractsWithActions;
    use InteractsWithForms;

    protected static ?string $slug = 'backups/restore/{backupId}';

    protected static bool $shouldRegisterNavigation = false;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-arrow-path';

    protected string $view = 'filament.admin.pages.restore-backup';

    // ── Wizard State ────────────────────────────────────────────────────

    public ?int $backupId = null;

    public int $step = 1;

    public ?string $selectedUser = null;

    public ?int $selectedBackupId = null;

    public array $contents = [];

    public string $activeSection = 'files';

    public string $currentPath = '';

    public array $selectedPaths = [];

    public array $selectedDatabases = [];

    public array $selectedMailboxes = [];

    public string $conflictMode = 'overwrite';

    private ?Backup $backup = null;

    // ── Mount ───────────────────────────────────────────────────────────

    public function mount(?int $backupId = null): void
    {
        $this->backupId = $backupId ?? (int) request()->route('backupId');
        $this->backup = Backup::find($this->backupId);

        if (! $this->backup || ! $this->backup->snapshot_id) {
            $this->redirect('/jabali-admin/backups');

            return;
        }

        $this->selectedBackupId = $this->backupId;
    }

    public function getTitle(): string|Htmlable
    {
        return __('Restore Wizard');
    }

    public function getSubheading(): ?string
    {
        return match ($this->step) {
            1 => __('Step 1: Select account and backup'),
            2 => __('Step 2: Select what to restore'),
            3 => __('Step 3: Browse and select items'),
            4 => __('Step 4: Confirm and restore'),
            default => '',
        };
    }

    // ── Header Actions ──────────────────────────────────────────────────

    protected function getHeaderActions(): array
    {
        return [
            Action::make('back')
                ->label(__('Back to Backups'))
                ->icon('heroicon-o-arrow-left')
                ->color('gray')
                ->url('/jabali-admin/backups'),
        ];
    }

    // ── Step Navigation ─────────────────────────────────────────────────

    public function nextStep(): void
    {
        if ($this->step === 1 && empty($this->selectedUser)) {
            Notification::make()->title(__('Please select a user'))->danger()->send();

            return;
        }

        if ($this->step === 1) {
            $this->loadContents();
        }

        if ($this->step === 2) {
            $this->currentPath = $this->selectedUser ?? '';
        }

        $this->step = min($this->step + 1, 4);
    }

    public function prevStep(): void
    {
        $this->step = max($this->step - 1, 1);
    }

    public function goToStep(int $step): void
    {
        if ($step <= $this->step) {
            $this->step = $step;
        }
    }

    // ── Step 2: Load Contents ───────────────────────────────────────────

    public function loadContents(): void
    {
        $backup = $this->getBackup();
        if (! $backup) {
            return;
        }

        try {
            $repo = $backup->destination
                ? $backup->destination->getResticRepoUrl()
                : '/var/backups/jabali/restic';
            $destConfig = $backup->destination
                ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
                : [];

            $agent = app(AgentClient::class);
            $result = $agent->send('backup.list_contents', [
                'snapshot_id' => $backup->snapshot_id,
                'destination' => $destConfig,
                'repo' => $repo,
            ]);

            $files = $result['files'] ?? [];
            $domains = [];
            $databases = [];
            $mailboxes = [];
            $dnsZones = [];
            $sslCerts = [];
            $hasDbUsers = false;

            foreach ($files as $file) {
                $parts = explode('/', $file);

                // Paths are: home/{user}/domains/{domain}/...
                if (str_contains($file, "home/{$this->selectedUser}/domains/") && count($parts) >= 5) {
                    $domIdx = array_search('domains', $parts);
                    if ($domIdx !== false && isset($parts[$domIdx + 1])) {
                        $domains[$parts[$domIdx + 1]] = true;
                    }
                } elseif (str_contains($file, "home/{$this->selectedUser}/.jabali-backup/databases/") && str_ends_with($file, '.sql.gz')) {
                    $databases[] = basename($file, '.sql.gz');
                } elseif (str_contains($file, "home/{$this->selectedUser}/.jabali-backup/databases/users.sql")) {
                    $hasDbUsers = true;
                } elseif (str_starts_with($file, 'var/mail/vhosts/') && count($parts) >= 5) {
                    $mailboxes["{$parts[4]}@{$parts[3]}"] = true;
                } elseif (str_contains($file, '/.jabali-backup/dns/') && str_ends_with($file, '.zone')) {
                    $dnsZones[] = basename($file, '.zone');
                } elseif (str_contains($file, '/letsencrypt/live/') && count($parts) >= 5) {
                    $sslCerts[$parts[4]] = true;
                }
            }

            $this->contents = [
                'domains' => array_keys($domains),
                'databases' => array_values(array_unique($databases)),
                'mailboxes' => array_keys($mailboxes),
                'dns_zones' => $dnsZones,
                'ssl_certs' => array_keys($sslCerts),
                'has_db_users' => $hasDbUsers,
                'total_files' => count($files),
            ];

            // Pre-select everything
            $this->selectedDatabases = $this->contents['databases'];
            $this->selectedMailboxes = $this->contents['mailboxes'];
        } catch (Exception $e) {
            $this->contents = [];
            Notification::make()->title(__('Failed to load contents'))->body(SafeError::message($e))->danger()->send();
        }
    }

    // ── Step 3: File Browser ────────────────────────────────────────────

    public function setSection(string $section): void
    {
        $this->activeSection = $section;
        $this->currentPath = $this->selectedUser ?? '';
    }

    public function navigateTo(string $path): void
    {
        $this->currentPath = $path;
    }

    public function loadDirectory(): array
    {
        $backup = $this->getBackup();
        if (! $backup || ! $this->selectedUser) {
            return [];
        }

        try {
            $adapter = $this->buildAdapter();
            $result = $adapter->files()->list($this->currentPath);

            return $result['items'] ?? [];
        } catch (Exception $e) {
            return [];
        }
    }

    // ── Step 4: Execute Restore ─────────────────────────────────────────

    public function executeRestore(): void
    {
        $user = User::where('username', $this->selectedUser)->first();
        if (! $user) {
            Notification::make()->title(__('User not found'))->danger()->send();

            return;
        }

        $backup = $this->getBackup();
        if (! $backup) {
            return;
        }

        try {
            $orchestrator = app(BackupOrchestrator::class);
            $result = $orchestrator->restoreBackup($user, $backup, [
                'restore_files' => ! empty($this->selectedPaths) || ! empty($this->contents['domains']),
                'restore_databases' => ! empty($this->selectedDatabases),
                'restore_mailboxes' => ! empty($this->selectedMailboxes),
                'selected_domains' => ! empty($this->selectedPaths)
                    ? array_filter($this->selectedPaths, fn ($p) => ! str_contains($p, '/'))
                    : ($this->contents['domains'] ?? null),
                'selected_databases' => $this->selectedDatabases ?: null,
                'selected_mailboxes' => $this->selectedMailboxes ?: null,
                'selected_files' => array_filter($this->selectedPaths, fn ($p) => str_contains($p, '/')),
            ]);

            if ($result['success'] ?? false) {
                Notification::make()
                    ->title(__('Restore completed'))
                    ->body(__(':files domain(s), :dbs database(s), :mail mailbox(es)', [
                        'files' => $result['result']['files_count'] ?? 0,
                        'dbs' => $result['result']['databases_count'] ?? 0,
                        'mail' => $result['result']['mailboxes_count'] ?? 0,
                    ]))
                    ->success()
                    ->send();

                $this->redirect('/jabali-admin/backups');
            } else {
                Notification::make()
                    ->title(__('Restore failed'))
                    ->body($result['error'] ?? __('Unknown error'))
                    ->danger()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Restore failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    // ── Helpers ──────────────────────────────────────────────────────────

    private function buildAdapter(): BackupSnapshotAdapter
    {
        $backup = $this->getBackup();
        $repo = $backup->destination
            ? $backup->destination->getResticRepoUrl()
            : '/var/backups/jabali/restic';
        $destConfig = $backup->destination
            ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
            : [];

        return new BackupSnapshotAdapter(
            app(AgentClient::class),
            $backup->snapshot_id,
            $this->selectedUser ?? 'admin',
            $repo,
            $destConfig,
        );
    }

    private function getBackup(): ?Backup
    {
        if ($this->backup === null) {
            $this->backup = Backup::find($this->backupId);
        }

        return $this->backup;
    }

    public function getAvailableBackups(): array
    {
        return Backup::where('status', 'completed')
            ->whereNotNull('snapshot_id')
            ->latest()
            ->pluck('name', 'id')
            ->toArray();
    }
}
