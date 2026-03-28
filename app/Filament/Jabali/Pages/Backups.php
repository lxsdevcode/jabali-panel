<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages;

use App\Models\BackupDestination;
use App\Services\Backup\BackupOrchestrator;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Components\Grid;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Auth;
use Livewire\Attributes\Url;

class Backups extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithForms;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-cloud-arrow-up';

    protected static ?int $navigationSort = 13;

    public static function getNavigationLabel(): string
    {
        return __('Backups');
    }

    protected string $view = 'filament.jabali.pages.backups';

    #[Url(as: 'tab')]
    public ?string $activeTab = 'backups';

    public function getTitle(): string|Htmlable
    {
        return __('Backups');
    }

    protected function getUser(): \App\Models\User
    {
        return Auth::user();
    }

    // ── Header Actions ──────────────────────────────────────────────────

    protected function getHeaderActions(): array
    {
        return [
            $this->addDestinationAction(),
        ];
    }

    // ── Destinations Table ──────────────────────────────────────────────

    public function table(Table $table): Table
    {
        return $table
            ->query(BackupDestination::where('user_id', $this->getUser()->id)->where('is_server_backup', false))
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->searchable(),
                TextColumn::make('type')
                    ->label(__('Type'))
                    ->badge()
                    ->formatStateUsing(fn (string $state) => match ($state) {
                        'sftp' => __('SFTP'),
                        's3' => __('S3'),
                        default => $state,
                    }),
                TextColumn::make('test_status')
                    ->label(__('Status'))
                    ->badge()
                    ->formatStateUsing(fn (?string $state) => match ($state) {
                        'success' => __('Connected'),
                        'failed' => __('Failed'),
                        default => __('Not tested'),
                    })
                    ->color(fn (?string $state) => match ($state) {
                        'success' => 'success',
                        'failed' => 'danger',
                        default => 'gray',
                    }),
                TextColumn::make('last_tested_at')
                    ->label(__('Last Tested'))
                    ->since()
                    ->placeholder(__('Never')),
            ])
            ->actions([
                Action::make('test')
                    ->label(__('Test'))
                    ->icon('heroicon-o-signal')
                    ->color('gray')
                    ->action(fn (BackupDestination $record) => $this->testDestination($record->id)),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->action(fn (BackupDestination $record) => $record->delete()),
            ])
            ->emptyStateHeading(__('No remote destinations configured'))
            ->emptyStateDescription(__('Click "Add Destination" to configure SFTP storage for your backups'))
            ->emptyStateIcon('heroicon-o-server-stack');
    }

    // ── Destination Actions ─────────────────────────────────────────────

    private function addDestinationAction(): Action
    {
        return Action::make('addDestination')
            ->label(__('Add SFTP'))
            ->icon('heroicon-o-plus')
            ->form([
                TextInput::make('name')
                    ->label(__('Name'))
                    ->placeholder(__('My Backup Server'))
                    ->required(),
                Grid::make(2)->schema([
                    TextInput::make('host')
                        ->label(__('Host'))
                        ->required(),
                    TextInput::make('port')
                        ->label(__('Port'))
                        ->numeric()
                        ->default(22),
                ]),
                TextInput::make('username')
                    ->label(__('Username'))
                    ->required(),
                TextInput::make('password')
                    ->label(__('Password'))
                    ->password(),
                Textarea::make('private_key')
                    ->label(__('SSH Private Key'))
                    ->rows(3)
                    ->helperText(__('Optional, alternative to password')),
                TextInput::make('path')
                    ->label(__('Remote Path'))
                    ->default('/backups'),
            ])
            ->action(function (array $data): void {
                $user = $this->getUser();

                $config = [
                    'type' => 'sftp',
                    'host' => $data['host'] ?? '',
                    'port' => (int) ($data['port'] ?? 22),
                    'username' => $data['username'] ?? '',
                    'password' => $data['password'] ?? '',
                    'private_key' => $data['private_key'] ?? '',
                    'path' => $data['path'] ?? '/backups',
                ];

                try {
                    $dest = BackupDestination::create([
                        'user_id' => $user->id,
                        'name' => $data['name'],
                        'type' => 'sftp',
                        'config' => $config,
                        'is_server_backup' => false,
                        'is_active' => true,
                    ]);

                    $orchestrator = app(BackupOrchestrator::class);
                    $orchestrator->testDestination($dest);

                    Notification::make()->title(__('Destination added'))->success()->send();
                } catch (Exception $e) {
                    Notification::make()
                        ->title(__('Failed'))
                        ->body(SafeError::message($e))
                        ->danger()
                        ->send();
                }
            });
    }

    public function testDestination(int $id): void
    {
        $user = $this->getUser();
        $destination = BackupDestination::where('id', $id)->where('user_id', $user->id)->first();

        if (! $destination) {
            return;
        }

        try {
            $orchestrator = app(BackupOrchestrator::class);
            $orchestrator->testDestination($destination);

            $destination->refresh();
            if ($destination->test_status === 'success') {
                Notification::make()->title(__('Connection successful'))->success()->send();
            } else {
                Notification::make()
                    ->title(__('Connection failed'))
                    ->body($destination->test_message ?? __('Unknown error'))
                    ->danger()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Connection failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }
}
