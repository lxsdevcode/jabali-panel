<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Filament\Concerns\ManagesNginxDirectives;
use App\Models\Domain;
use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Services\DomainHealthService;
use BackedEnum;
use Filament\Actions\Action;
use Filament\Actions\ActionGroup;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Support\Enums\Width;
use Filament\Tables\Columns\IconColumn;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Filters\SelectFilter;
use Filament\Tables\Filters\TernaryFilter;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Log;

class Domains extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithForms;
    use InteractsWithTable;
    use ManagesNginxDirectives;

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
                    ->url(fn (Domain $record): string => "https://{$record->domain}", shouldOpenInNewTab: true),
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
                    ->sortable()
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
                    ->sortable()
                    ->toggleable()
                    ->formatStateUsing(function (?string $state, Domain $record): string {
                        $label = match ($state) {
                            'registered' => __('Registered'),
                            'expired' => __('Expired'),
                            'unregistered' => __('Unregistered'),
                            'whois_error' => __('Error'),
                            default => __('Unchecked'),
                        };

                        if ($record->whois_expiry) {
                            $label .= ' · '.$record->whois_expiry->format('Y-m-d');
                        }

                        return $label;
                    })
                    ->color(fn (?string $state): string => match ($state) {
                        'registered' => 'success',
                        'expired' => 'danger',
                        'whois_error' => 'warning',
                        default => 'gray',
                    }),
                TextColumn::make('bandwidth')
                    ->label(__('Bandwidth'))
                    ->state(fn (Domain $record): string => \App\Support\Formatter::bytes($record->currentMonthBandwidth()))
                    ->badge()
                    ->color(function (Domain $record): string {
                        $limit = $record->user?->hostingPackage?->bandwidth_gb;
                        if (! $limit) {
                            return 'gray';
                        }
                        $pct = $record->currentMonthBandwidth() / ($limit * 1073741824) * 100;

                        return $pct >= 95 ? 'danger' : ($pct >= 80 ? 'warning' : 'success');
                    })
                    ->toggleable(),
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
                ActionGroup::make([
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
                    Action::make('toggleDomain')
                        ->label(fn (Domain $record): string => $record->is_active ? __('Disable') : __('Enable'))
                        ->icon(fn (Domain $record): string => $record->is_active ? 'heroicon-o-pause-circle' : 'heroicon-o-play-circle')
                        ->color(fn (Domain $record): string => $record->is_active ? 'warning' : 'success')
                        ->requiresConfirmation()
                        ->action(fn (Domain $record) => $this->toggleDomain($record)),
                    Action::make('domainInfo')
                        ->label(__('Domain Info'))
                        ->icon('heroicon-o-information-circle')
                        ->color('info')
                        ->modalHeading(fn (Domain $record): string => $record->domain)
                        ->modalContent(function (Domain $record) {
                            $whoisData = app(DomainHealthService::class)->checkWhois($record);

                            return view('filament.admin.pages.domain-info-modal', [
                                'domain' => $record,
                                'whoisData' => $whoisData,
                            ]);
                        })
                        ->modalSubmitAction(false)
                        ->modalCancelActionLabel(__('Close'))
                        ->modalWidth('3xl'),
                    $this->nginxDirectivesAction()
                        ->modalHeading(fn (Domain $record) => __('Nginx Directives for :domain', ['domain' => $record->domain]))
                        ->modalWidth(Width::FourExtraLarge),
                    Action::make('deleteDomain')
                        ->label(__('Delete'))
                        ->icon('heroicon-o-trash')
                        ->color('danger')
                        ->requiresConfirmation()
                        ->modalHeading(fn (Domain $record): string => __('Delete :domain', ['domain' => $record->domain]))
                        ->modalDescription(__('This will permanently delete the domain, its DNS records, SSL certificates, and all associated data. This cannot be undone.'))
                        ->action(fn (Domain $record) => $this->deleteDomain($record)),
                ]),
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
                Action::make('checkAllWhois')
                    ->label(__('Check All Registration'))
                    ->icon('heroicon-o-document-magnifying-glass')
                    ->color('gray')
                    ->requiresConfirmation()
                    ->modalHeading(__('Check All Registration'))
                    ->modalDescription(__('This will run WHOIS lookups for all domains. This may take a while.'))
                    ->action(fn () => $this->checkAllWhois()),
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

    public function checkAllWhois(): void
    {
        $service = app(DomainHealthService::class);
        $domains = Domain::all();

        foreach ($domains as $domain) {
            $result = $service->checkWhois($domain);
            $domain->update([
                'whois_status' => $result['status'],
                'whois_expiry' => $result['expiry'],
            ]);
        }

        Notification::make()
            ->title(__('Registration Check Complete'))
            ->body(__('All domains have been checked.'))
            ->success()
            ->send();
    }

    public function toggleDomain(Domain $domain): void
    {
        $domain->update(['is_active' => ! $domain->is_active]);

        if ($domain->is_active) {
            try {
                app(AgentClient::class)->send('domain.enable', ['domain' => $domain->domain]);
            } catch (\Throwable $e) {
                Log::warning("Failed to enable domain {$domain->domain}: {$e->getMessage()}");
            }
        } else {
            try {
                app(AgentClient::class)->send('domain.disable', ['domain' => $domain->domain]);
            } catch (\Throwable $e) {
                Log::warning("Failed to disable domain {$domain->domain}: {$e->getMessage()}");
            }
        }

        Notification::make()
            ->title($domain->is_active ? __('Domain enabled') : __('Domain disabled'))
            ->body($domain->domain)
            ->success()
            ->send();
    }

    public function deleteDomain(Domain $domain): void
    {
        try {
            app(AgentClient::class)->domainDelete($domain->user->username, $domain->domain);
        } catch (\Throwable $e) {
            Log::warning("Failed to delete domain {$domain->domain} from server: {$e->getMessage()}");
        }

        $domain->delete();

        Notification::make()
            ->title(__('Domain deleted'))
            ->body($domain->domain)
            ->success()
            ->send();
    }
}
