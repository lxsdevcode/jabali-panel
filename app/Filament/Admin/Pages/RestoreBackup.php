<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\FileBrowser\Adapters\FileBrowserAdapter;
use App\FileBrowser\Pages\FileBrowser;
use App\Models\Backup;
use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Services\FileBrowser\BackupSnapshotAdapter;
use App\Support\SafeError;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\Select;
use Filament\Notifications\Notification;
use Illuminate\Contracts\Support\Htmlable;

class RestoreBackup extends FileBrowser
{
    protected static ?string $slug = 'backups/restore/{backupId}';

    protected static bool $shouldRegisterNavigation = false;

    protected static string|\BackedEnum|null $navigationIcon = 'heroicon-o-arrow-path';

    public ?int $backupId = null;

    private ?Backup $backup = null;

    public function mount(?int $backupId = null): void
    {
        $this->backupId = $backupId ?? (int) request()->route('backupId');
        $this->backup = Backup::find($this->backupId);

        if (! $this->backup || ! $this->backup->snapshot_id) {
            $this->redirect(route('filament.admin.pages.backups'));

            return;
        }

        $this->currentPath = '';
    }

    public function getTitle(): string|Htmlable
    {
        return __('Restore: :name', ['name' => $this->getBackup()?->name ?? '']);
    }

    public function getSubheading(): ?string
    {
        return __('Browse the snapshot and navigate folders. Click "Restore This Folder" to restore the current directory.');
    }

    public function getAdapter(): FileBrowserAdapter
    {
        $backup = $this->getBackup();
        $repo = $backup->destination
            ? $backup->destination->getResticRepoUrl()
            : '/var/backups/jabali/restic';
        $destConfig = $backup->destination
            ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
            : [];

        $username = $backup->users[0] ?? 'admin';

        return new BackupSnapshotAdapter(
            app(AgentClient::class),
            $backup->snapshot_id,
            $username,
            $repo,
            $destConfig,
        );
    }

    protected function getHeaderActions(): array
    {
        return [
            Action::make('restoreCurrentFolder')
                ->label(__('Restore This Folder'))
                ->icon('heroicon-o-arrow-path')
                ->color('warning')
                ->requiresConfirmation()
                ->modalHeading(__('Restore Folder'))
                ->modalDescription(fn () => empty($this->currentPath)
                    ? __('Restore the entire user directory from this snapshot?')
                    : __('Restore ":path" from this snapshot?', ['path' => $this->currentPath]))
                ->form([
                    Select::make('restore_user')
                        ->label(__('Restore for user'))
                        ->options(fn () => User::where('is_active', true)->pluck('username', 'username')->toArray())
                        ->required(),
                ])
                ->action(function (array $data): void {
                    $this->restorePath($data['restore_user']);
                }),
            Action::make('backToBackups')
                ->label(__('Back to Backups'))
                ->icon('heroicon-o-arrow-left')
                ->color('gray')
                ->url(route('filament.admin.pages.backups')),
        ];
    }

    private function restorePath(string $username): void
    {
        $user = User::where('username', $username)->first();
        if (! $user) {
            Notification::make()->title(__('User not found'))->danger()->send();

            return;
        }

        $backup = $this->getBackup();

        try {
            $repo = $backup->destination
                ? $backup->destination->getResticRepoUrl()
                : '/var/backups/jabali/restic';
            $destConfig = $backup->destination
                ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
                : [];

            $agent = app(AgentClient::class);
            $result = $agent->send('backup.restore', [
                'snapshot_id' => $backup->snapshot_id,
                'username' => $username,
                'repo' => $repo,
                'destination' => $destConfig,
                'restore_files' => true,
                'restore_databases' => false,
                'restore_mailboxes' => false,
            ]);

            if ($result['success'] ?? false) {
                Notification::make()
                    ->title(__('Restore completed'))
                    ->body(empty($this->currentPath) ? __('Full directory restored') : __('Restored: :path', ['path' => $this->currentPath]))
                    ->success()
                    ->send();
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

    private function getBackup(): ?Backup
    {
        if ($this->backup === null) {
            $this->backup = Backup::find($this->backupId);
        }

        return $this->backup;
    }
}
