<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Models\Domain;
use App\Models\User;
use App\Services\DomainHealthService;
use BackedEnum;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Tables\Columns\IconColumn;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Filters\TernaryFilter;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Artisan;

class Domains extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithForms;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-globe-alt';

    protected static ?int $navigationSort = 2;

    protected string $view = 'filament.admin.pages.domains';

    public static function getNavigationLabel(): string
    {
        return __('Domains');
    }

    public function getTitle(): string|Htmlable
    {
        return __('Domains');
    }

    public function table(Table $table): Table
    {
        return $table
            ->query(Domain::query()->with('user'))
            ->columns([
                TextColumn::make('domain')
                    ->label(__('Domain'))
                    ->searchable()
                    ->sortable()
                    ->description(fn (Domain $record): string => $record->document_root ?? ''),
                TextColumn::make('user.username')
                    ->label(__('Owner'))
                    ->searchable()
                    ->sortable(),
                IconColumn::make('is_active')
                    ->label(__('Active'))
                    ->boolean()
                    ->sortable(),
                IconColumn::make('ssl_enabled')
                    ->label(__('SSL'))
                    ->boolean(),
                TextColumn::make('dns_status')
                    ->label(__('DNS Status'))
                    ->badge()
                    ->formatStateUsing(fn (?string $state): string => match ($state) {
                        'points_here' => __('Points Here'),
                        'cloudflare' => __('Cloudflare'),
                        'external' => __('External'),
                        'dns_missing' => __('DNS Missing'),
                        'dns_error' => __('DNS Error'),
                        default => __('Unchecked'),
                    })
                    ->color(fn (?string $state): string => match ($state) {
                        'points_here' => 'success',
                        'cloudflare' => 'info',
                        'external' => 'warning',
                        'dns_error' => 'danger',
                        default => 'gray',
                    }),
                TextColumn::make('whois_status')
                    ->label(__('Registration'))
                    ->badge()
                    ->toggleable()
                    ->formatStateUsing(fn (?string $state): string => match ($state) {
                        'registered' => __('Registered'),
                        'expired' => __('Expired'),
                        'unregistered' => __('Unregistered'),
                        'whois_error' => __('Error'),
                        default => __('Unchecked'),
                    })
                    ->color(fn (?string $state): string => match ($state) {
                        'registered' => 'success',
                        'expired' => 'danger',
                        'whois_error' => 'warning',
                        default => 'gray',
                    }),
                TextColumn::make('dns_checked_at')
                    ->label(__('Last DNS Check'))
                    ->since()
                    ->toggleable(isToggledHiddenByDefault: true),
                TextColumn::make('created_at')
                    ->label(__('Created'))
                    ->date()
                    ->sortable()
                    ->toggleable(isToggledHiddenByDefault: true),
            ])
            ->filters([
                TernaryFilter::make('is_active')
                    ->label(__('Active')),
                SelectFilter::make('dns_status')
                    ->label(__('DNS Status'))
                    ->options([
                        'points_here' => __('Points Here'),
                        'cloudflare' => __('Cloudflare'),
                        'external' => __('External'),
                        'dns_missing' => __('DNS Missing'),
                        'dns_error' => __('DNS Error'),
                    ]),
                SelectFilter::make('user_id')
                    ->label(__('Owner'))
                    ->options(fn () => User::orderBy('username')->pluck('username', 'id')->toArray())
                    ->searchable()
                    ->preload(),
            ])
            ->recordActions([
                Action::make('checkDns')
                    ->label(__('Check DNS'))
                    ->icon('heroicon-o-magnifying-glass')
                    ->color('gray')
                    ->action(fn (Domain $record) => $this->checkDns($record)),
                Action::make('checkWhois')
                    ->label(__('Check WHOIS'))
                    ->icon('heroicon-o-document-magnifying-glass')
                    ->color('gray')
                    ->action(fn (Domain $record) => $this->checkWhois($record)),
                Action::make('domainInfo')
                    ->label(__('Domain Info'))
                    ->icon('heroicon-o-information-circle')
                    ->color('info')
                    ->modalHeading(fn (Domain $record): string => $record->domain)
                    ->modalContent(fn (Domain $record) => view('filament.admin.pages.domain-info-modal', [
                        'domain' => $record,
                    ]))
                    ->modalSubmitAction(false)
                    ->modalCancelActionLabel(__('Close')),
            ])
            ->headerActions([
                Action::make('checkAllDns')
                    ->label(__('Check All DNS'))
                    ->icon('heroicon-o-arrow-path')
                    ->color('gray')
                    ->requiresConfirmation()
                    ->modalHeading(__('Check All DNS'))
                    ->modalDescription(__('This will check DNS resolution for all domains. This may take a while.'))
                    ->action(fn () => $this->checkAllDns()),
            ])
            ->defaultSort('domain', 'asc')
            ->heading(__('Domains'));
    }

    public function checkDns(Domain $domain): void
    {
        $service = app(DomainHealthService::class);
        $result = $service->checkDns($domain);

        $domain->update([
            'dns_status' => $result['status'],
            'dns_resolved_ip' => $result['resolved_ip'],
            'dns_checked_at' => now(),
        ]);

        $label = match ($result['status']) {
            'points_here' => __('Points Here'),
            'cloudflare' => __('Cloudflare'),
            'external' => __('External'),
            'dns_missing' => __('DNS Missing'),
            'dns_error' => __('DNS Error'),
            default => $result['status'],
        };

        Notification::make()
            ->title(__('DNS Check Complete'))
            ->body(__(':domain: :status', ['domain' => $domain->domain, 'status' => $label]))
            ->success()
            ->send();
    }

    public function checkWhois(Domain $domain): void
    {
        $service = app(DomainHealthService::class);
        $result = $service->checkWhois($domain);

        $domain->update([
            'whois_status' => $result['status'],
            'whois_expiry' => $result['expiry'],
        ]);

        $label = match ($result['status']) {
            'registered' => __('Registered'),
            'expired' => __('Expired'),
            'unregistered' => __('Unregistered'),
            'whois_error' => __('Error'),
            default => $result['status'],
        };

        $body = __(':domain: :status', ['domain' => $domain->domain, 'status' => $label]);
        if ($result['expiry']) {
            $body .= ' — '.__('Expires: :date', ['date' => $result['expiry']]);
        }
        if ($result['registrar']) {
            $body .= ' ('.$result['registrar'].')';
        }

        Notification::make()
            ->title(__('WHOIS Check Complete'))
            ->body($body)
            ->success()
            ->send();
    }

    public function checkAllDns(): void
    {
        Artisan::call('jabali:check-domain-dns');

        Notification::make()
            ->title(__('DNS Check Complete'))
            ->body(__('All domains have been checked.'))
            ->success()
            ->send();
    }
}
