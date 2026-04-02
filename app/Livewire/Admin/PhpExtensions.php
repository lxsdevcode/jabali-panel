<?php

declare(strict_types=1);

namespace App\Livewire\Admin;

use App\Services\Agent\AgentResult;
use App\Services\Agent\InteractsWithAgent;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Tables\Columns\IconColumn;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\View\View;
use Illuminate\Database\Eloquent\Model;
use Livewire\Component;

class PhpExtensions extends Component implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithForms;
    use InteractsWithTable;

    private const VERSION_PATTERN = '/^\d+\.\d+$/';

    private const EXTENSION_PATTERN = '/^[a-zA-Z][a-zA-Z0-9_]*$/';

    public string $version = '';

    public array $extensions = [];

    public function mount(string $version): void
    {
        if (! preg_match(self::VERSION_PATTERN, $version)) {
            $this->version = '';
            $this->extensions = [];

            return;
        }

        $this->version = $version;
        $this->loadExtensions();
    }

    public function loadExtensions(): void
    {
        if ($this->version === '') {
            $this->extensions = [];

            return;
        }

        try {
            $response = $this->agent()->send('php.list_extensions', ['version' => $this->version]);
            $result = AgentResult::fromResponse($response);
            $installed = $result->success ? $result->get('extensions', []) : [];
        } catch (\Exception) {
            $installed = [];
        }

        // Mark all agent-returned extensions as installed
        $installedMap = [];
        foreach ($installed as &$ext) {
            $ext['installed'] = true;
            $installedMap[$ext['name']] = true;
        }
        unset($ext);

        // Merge with known extensions from config (add uninstalled ones)
        /** @var array<string, bool> $knownExtensions */
        $knownExtensions = config('jabali.php.extensions', []);
        foreach ($knownExtensions as $name => $isDefault) {
            if (! isset($installedMap[$name])) {
                $installed[] = ['name' => $name, 'enabled' => false, 'installed' => false];
            }
        }

        usort($installed, fn (array $a, array $b): int => strcmp($a['name'], $b['name']));
        $this->extensions = $installed;
    }

    public function enableExtension(string $extension): void
    {
        if (! $this->validateExtension($extension)) {
            return;
        }

        $this->agentCall(
            action: 'php.enable_extension',
            params: ['version' => $this->version, 'extension' => $extension],
            successTitle: __('Extension :ext enabled', ['ext' => $extension]),
            errorTitle: __('Failed to enable :ext', ['ext' => $extension]),
            onSuccess: function (): void {
                $this->loadExtensions();
                $this->resetTable();
            },
        );
    }

    public function disableExtension(string $extension): void
    {
        if (! $this->validateExtension($extension)) {
            return;
        }

        $this->agentCall(
            action: 'php.disable_extension',
            params: ['version' => $this->version, 'extension' => $extension],
            successTitle: __('Extension :ext disabled', ['ext' => $extension]),
            errorTitle: __('Failed to disable :ext', ['ext' => $extension]),
            onSuccess: function (): void {
                $this->loadExtensions();
                $this->resetTable();
            },
        );
    }

    public function installExtension(string $extension): void
    {
        if (! $this->validateExtension($extension)) {
            return;
        }

        $this->agentCall(
            action: 'php.install_extension',
            params: ['version' => $this->version, 'extension' => $extension],
            successTitle: __('Extension :ext installed', ['ext' => $extension]),
            errorTitle: __('Failed to install :ext', ['ext' => $extension]),
            onSuccess: function (): void {
                $this->loadExtensions();
                $this->resetTable();
            },
        );
    }

    public function removeExtension(string $extension): void
    {
        if (! $this->validateExtension($extension)) {
            return;
        }

        $this->agentCall(
            action: 'php.remove_extension',
            params: ['version' => $this->version, 'extension' => $extension],
            successTitle: __('Extension :ext removed', ['ext' => $extension]),
            errorTitle: __('Failed to remove :ext', ['ext' => $extension]),
            onSuccess: function (): void {
                $this->loadExtensions();
                $this->resetTable();
            },
        );
    }

    private function validateExtension(string $extension): bool
    {
        if (! preg_match(self::EXTENSION_PATTERN, $extension)) {
            Notification::make()
                ->title(__('Invalid extension name'))
                ->danger()
                ->send();

            return false;
        }

        return true;
    }

    public function table(Table $table): Table
    {
        return $table
            ->records(fn () => $this->extensions)
            ->columns([
                TextColumn::make('name')
                    ->label(__('Extension'))
                    ->icon('heroicon-o-puzzle-piece')
                    ->searchable(),
                IconColumn::make('enabled')
                    ->label(__('Status'))
                    ->boolean()
                    ->trueIcon('heroicon-o-check-circle')
                    ->falseIcon('heroicon-o-x-circle')
                    ->trueColor('success')
                    ->falseColor('gray'),
                TextColumn::make('installed')
                    ->label(__('Installed'))
                    ->badge()
                    ->formatStateUsing(fn (bool $state): string => $state ? __('Yes') : __('No'))
                    ->color(fn (bool $state): string => $state ? 'success' : 'gray'),
            ])
            ->recordActions([
                Action::make('enable')
                    ->label(__('Enable'))
                    ->icon('heroicon-o-check')
                    ->color('success')
                    ->size('sm')
                    ->visible(fn (array $record): bool => $record['installed'] && ! $record['enabled'])
                    ->action(fn (array $record) => $this->enableExtension($record['name'])),
                Action::make('disable')
                    ->label(__('Disable'))
                    ->icon('heroicon-o-x-mark')
                    ->color('warning')
                    ->size('sm')
                    ->visible(fn (array $record): bool => $record['installed'] && $record['enabled'])
                    ->requiresConfirmation()
                    ->action(fn (array $record) => $this->disableExtension($record['name'])),
                Action::make('install')
                    ->label(__('Install'))
                    ->icon('heroicon-o-arrow-down-tray')
                    ->color('primary')
                    ->size('sm')
                    ->visible(fn (array $record): bool => ! $record['installed'])
                    ->action(fn (array $record) => $this->installExtension($record['name'])),
                Action::make('remove')
                    ->label(__('Remove'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->size('sm')
                    ->visible(fn (array $record): bool => $record['installed'] && ! $record['enabled'])
                    ->requiresConfirmation()
                    ->action(fn (array $record) => $this->removeExtension($record['name'])),
            ])
            ->heading(__('PHP :version Extensions', ['version' => $this->version]))
            ->striped();
    }

    public function getTableRecordKey(Model|array $record): string
    {
        return is_array($record) ? $record['name'] : (string) $record->getKey();
    }

    public function render(): View
    {
        return view('livewire.admin.php-extensions');
    }
}
