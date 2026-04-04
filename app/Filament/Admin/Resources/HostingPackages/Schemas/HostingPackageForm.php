<?php

declare(strict_types=1);

namespace App\Filament\Admin\Resources\HostingPackages\Schemas;

use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Schemas\Components\Section;
use Filament\Schemas\Components\Utilities\Get;
use Filament\Schemas\Schema;

class HostingPackageForm
{
    public static function configure(Schema $schema): Schema
    {
        return $schema
            ->columns(1)
            ->components([
                Section::make(__('Package Details'))
                    ->schema([
                        TextInput::make('name')
                            ->label(__('Name'))
                            ->required()
                            ->maxLength(120)
                            ->unique(ignoreRecord: true),
                        Textarea::make('description')
                            ->label(__('Description'))
                            ->rows(3)
                            ->columnSpanFull(),
                        Toggle::make('is_active')
                            ->label(__('Active'))
                            ->default(true),
                        Toggle::make('ssh_shell_enabled')
                            ->label(__('SSH Shell Access'))
                            ->helperText(__('New accounts with this package get SSH shell access instead of SFTP-only'))
                            ->default(false)
                            ->live(),
                        Select::make('ssh_isolation_mode')
                            ->label(__('SSH Isolation Mode'))
                            ->options([
                                'container' => __('Container (nspawn) — full isolation'),
                                'sandbox' => __('Sandbox (bwrap) — lighter, IDE-compatible'),
                                'standard' => __('Standard — plain shell, no isolation'),
                            ])
                            ->default('container')
                            ->visible(fn (Get $get): bool => (bool) $get('ssh_shell_enabled'))
                            ->helperText(__('Container is most secure. Sandbox works better with VS Code Remote SSH. Standard has no isolation.')),
                    ])
                    ->columns(2),

                Section::make(__('Resource Limits'))
                    ->description(__('Leave blank for unlimited.'))
                    ->schema([
                        TextInput::make('disk_quota_mb')
                            ->label(__('Disk Quota (MB)'))
                            ->numeric()
                            ->minValue(0)
                            ->helperText(__('Example: 10240 = 10 GB')),
                        TextInput::make('bandwidth_gb')
                            ->label(__('Bandwidth (GB / month)'))
                            ->numeric()
                            ->minValue(0),
                        TextInput::make('domains_limit')
                            ->label(__('Domains Limit'))
                            ->numeric()
                            ->minValue(0),
                        TextInput::make('databases_limit')
                            ->label(__('Databases Limit'))
                            ->numeric()
                            ->minValue(0),
                        TextInput::make('mailboxes_limit')
                            ->label(__('Mailboxes Limit'))
                            ->numeric()
                            ->minValue(0),
                    ])
                    ->columns(2),

            ]);
    }
}
