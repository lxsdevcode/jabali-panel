<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Services\Agent\InteractsWithAgent;
use App\Support\SafeError;
use BackedEnum;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Support\Icons\Heroicon;
use Illuminate\Contracts\Support\Htmlable;
use Livewire\Attributes\Url;

class SystemLogs extends Page implements HasActions, HasForms
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithForms;

    protected static string|BackedEnum|null $navigationIcon = Heroicon::OutlinedDocumentText;

    protected static ?int $navigationSort = 11;

    protected static ?string $slug = 'system-logs';

    protected string $view = 'filament.admin.pages.system-logs';

    #[Url(as: 'tab')]
    public string $activeTab = 'panel';

    #[Url(as: 'log')]
    public string $selectedLog = 'jabali-panel';

    public int $lineCount = 200;

    public string $logContent = '';

    public int $fileSize = 0;

    public ?string $lastModified = null;

    public ?string $errorMessage = null;

    public function getTitle(): string|Htmlable
    {
        return __('System Logs');
    }

    public static function getNavigationLabel(): string
    {
        return __('System Logs');
    }

    public function mount(): void
    {
        $this->activeTab = $this->normalizeTab($this->activeTab);
        $this->selectedLog = $this->normalizeLog($this->selectedLog, $this->activeTab);
        $this->loadLog();
    }

    // ── Header Actions ─────────────────────────────────────────────────

    protected function getHeaderActions(): array
    {
        return [
            Action::make('refresh')
                ->label(__('Refresh'))
                ->icon('heroicon-o-arrow-path')
                ->color('gray')
                ->action(fn () => $this->loadLog()),
        ];
    }

    // ── Tab / Log Config ───────────────────────────────────────────────

    public static function getLogTabs(): array
    {
        return [
            'panel' => [
                'label' => 'Panel',
                'icon' => 'heroicon-o-server-stack',
                'logs' => [
                    'jabali-panel' => 'Panel (stdout)',
                    'jabali-panel-err' => 'Panel (stderr)',
                    'jabali-agent' => 'Agent (stdout)',
                    'jabali-agent-err' => 'Agent (stderr)',
                    'jabali-agent-app' => 'Agent App Log',
                    'queue-worker' => 'Queue Worker',
                    'queue-worker-err' => 'Queue Worker (stderr)',
                ],
            ],
            'web' => [
                'label' => 'Web',
                'icon' => 'heroicon-o-globe-alt',
                'logs' => [
                    'nginx' => 'Nginx (supervisor)',
                    'nginx-err' => 'Nginx (supervisor err)',
                    'nginx-access' => 'Nginx Access Log',
                    'nginx-error' => 'Nginx Error Log',
                ],
            ],
            'mail' => [
                'label' => 'Mail',
                'icon' => 'heroicon-o-envelope',
                'logs' => [
                    'postfix' => 'Postfix',
                    'postfix-err' => 'Postfix (stderr)',
                    'dovecot' => 'Dovecot',
                    'dovecot-err' => 'Dovecot (stderr)',
                    'rspamd' => 'Rspamd',
                    'rspamd-err' => 'Rspamd (stderr)',
                    'opendkim' => 'OpenDKIM',
                    'opendkim-err' => 'OpenDKIM (stderr)',
                    'stalwart' => 'Stalwart',
                    'stalwart-err' => 'Stalwart (stderr)',
                ],
            ],
            'database' => [
                'label' => 'Database',
                'icon' => 'heroicon-o-circle-stack',
                'logs' => [
                    'mariadb' => 'MariaDB',
                    'mariadb-err' => 'MariaDB (stderr)',
                    'redis' => 'Redis',
                    'redis-err' => 'Redis (stderr)',
                ],
            ],
            'dns' => [
                'label' => 'DNS',
                'icon' => 'heroicon-o-globe-americas',
                'logs' => [
                    'pdns' => 'PowerDNS',
                    'pdns-err' => 'PowerDNS (stderr)',
                ],
            ],
            'security' => [
                'label' => 'Security',
                'icon' => 'heroicon-o-shield-check',
                'logs' => [
                    'fail2ban' => 'Fail2Ban',
                    'fail2ban-err' => 'Fail2Ban (stderr)',
                ],
            ],
            'laravel' => [
                'label' => 'Laravel',
                'icon' => 'heroicon-o-code-bracket',
                'logs' => [
                    'laravel' => 'Application Log',
                ],
            ],
        ];
    }

    // ── Actions ────────────────────────────────────────────────────────

    public function setTab(string $tab): void
    {
        $tab = $this->normalizeTab($tab);
        if ($this->activeTab === $tab) {
            return;
        }

        $this->activeTab = $tab;
        $tabs = self::getLogTabs();
        $firstLog = array_key_first($tabs[$tab]['logs'] ?? []);
        $this->selectedLog = $firstLog ?? '';
        $this->loadLog();
    }

    public function setLog(string $log): void
    {
        $log = $this->normalizeLog($log, $this->activeTab);
        if ($this->selectedLog === $log) {
            return;
        }

        $this->selectedLog = $log;
        $this->loadLog();
    }

    public function setLineCount(int $count): void
    {
        $this->lineCount = in_array($count, [100, 200, 500, 1000]) ? $count : 200;
        $this->loadLog();
    }

    public function loadLog(): void
    {
        $this->errorMessage = null;

        if (empty($this->selectedLog)) {
            $this->logContent = '';
            $this->fileSize = 0;
            $this->lastModified = null;

            return;
        }

        try {
            $result = $this->agent()->call('system.read_log', [
                'log_name' => $this->selectedLog,
                'lines' => $this->lineCount,
            ]);

            $this->logContent = $result->get('content', '');
            $this->fileSize = (int) $result->get('file_size', 0);
            $this->lastModified = $result->get('last_modified');
            $this->errorMessage = $result->get('error_message');
        } catch (\Exception $e) {
            $this->logContent = '';
            $this->fileSize = 0;
            $this->lastModified = null;

            Notification::make()
                ->title(__('Failed to load log'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    // ── Helpers ─────────────────────────────────────────────────────────

    private function normalizeTab(string $tab): string
    {
        $tabs = self::getLogTabs();

        return isset($tabs[$tab]) ? $tab : 'panel';
    }

    private function normalizeLog(string $log, string $tab): string
    {
        $tabs = self::getLogTabs();
        $logs = $tabs[$tab]['logs'] ?? [];

        if (isset($logs[$log])) {
            return $log;
        }

        return (string) array_key_first($logs);
    }

    public function formatFileSize(int $bytes): string
    {
        if ($bytes === 0) {
            return '0 B';
        }

        $units = ['B', 'KB', 'MB', 'GB'];
        $i = 0;
        $size = (float) $bytes;

        while ($size >= 1024 && $i < count($units) - 1) {
            $size /= 1024;
            $i++;
        }

        return round($size, 1).' '.$units[$i];
    }
}
