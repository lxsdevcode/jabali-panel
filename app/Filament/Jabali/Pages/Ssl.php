<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages;

use App\Models\Domain;
use App\Models\SslCertificate;
use App\Services\SslManagementService;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Tables\Columns\IconColumn;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\Facades\Auth;

class Ssl extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithForms;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-lock-closed';

    protected static ?int $navigationSort = 10;

    protected string $view = 'filament.jabali.pages.ssl';

    public static function getNavigationLabel(): string
    {
        return __('SSL Certificates');
    }

    public function getTitle(): string|Htmlable
    {
        return __('SSL Certificates');
    }

    public function getUsername(): string
    {
        return Auth::user()->username;
    }

    public function table(Table $table): Table
    {
        return $table
            ->query(
                SslCertificate::query()
                    ->whereHas('domain', fn ($q) => $q->where('user_id', Auth::id()))
                    ->with('domain')
            )
            ->columns([
                TextColumn::make('domain.domain')
                    ->label(__('Domain'))
                    ->formatStateUsing(function ($state, SslCertificate $record) {
                        if ($record->service === 'mail') {
                            return 'mail.'.$state;
                        }

                        return $state;
                    })
                    ->icon(fn (SslCertificate $record) => $record->isActive() ? 'heroicon-o-lock-closed' : 'heroicon-o-lock-open')
                    ->iconColor(fn (SslCertificate $record) => $record->status_color ?? 'gray')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('service')
                    ->label(__('Service'))
                    ->badge()
                    ->color(fn (string $state) => match ($state) {
                        'web' => 'info',
                        'mail' => 'warning',
                        default => 'gray',
                    })
                    ->formatStateUsing(fn (string $state) => match ($state) {
                        'web' => __('HTTPS'),
                        'mail' => __('Mail (IMAPS/SMTPS)'),
                        default => ucfirst($state),
                    }),
                TextColumn::make('type')
                    ->label(__('Type'))
                    ->badge()
                    ->formatStateUsing(fn (?string $state) => match ($state) {
                        'lets_encrypt' => __("Let's Encrypt"),
                        'self_signed' => __('Self-Signed'),
                        'custom' => __('Custom'),
                        default => __('None'),
                    })
                    ->color('gray'),
                TextColumn::make('status')
                    ->label(__('Status'))
                    ->badge()
                    ->getStateUsing(fn (SslCertificate $record) => $record->status_label ?? __('Unknown'))
                    ->color(fn (SslCertificate $record) => $record->status_color ?? 'gray'),
                TextColumn::make('expires_at')
                    ->label(__('Expires'))
                    ->date('M d, Y')
                    ->description(fn (SslCertificate $record) => $record->days_until_expiry !== null
                        ? ($record->days_until_expiry < 0
                            ? __('Expired :days days ago', ['days' => abs($record->days_until_expiry)])
                            : __(':days days left', ['days' => $record->days_until_expiry]))
                        : null)
                    ->placeholder(__('-'))
                    ->sortable(),
                IconColumn::make('auto_renew')
                    ->label(__('Auto-Renew'))
                    ->boolean()
                    ->trueIcon('heroicon-o-check-circle')
                    ->falseIcon('heroicon-o-x-circle')
                    ->trueColor('success')
                    ->falseColor('gray')
                    ->action(fn (SslCertificate $record) => $record->service === 'web' && $record->type === 'lets_encrypt' ? $this->toggleAutoRenew($record->domain->domain) : null),
            ])
            ->recordActions([
                Action::make('issueSsl')
                    ->label(__('Issue SSL'))
                    ->icon('heroicon-o-shield-check')
                    ->color('success')
                    ->visible(fn (SslCertificate $record) => $record->service === 'web' && ($record->type === 'self_signed' || in_array($record->status, ['pending', 'failed'])))
                    ->requiresConfirmation()
                    ->modalHeading(__('Issue SSL Certificate'))
                    ->modalDescription(fn (SslCertificate $record) => __("Issue a free Let's Encrypt SSL certificate for :domain? This will enable HTTPS for your domain.", ['domain' => $record->domain->domain]))
                    ->modalIcon('heroicon-o-shield-check')
                    ->modalIconColor('success')
                    ->modalSubmitActionLabel(__('Issue Certificate'))
                    ->action(fn (SslCertificate $record) => $this->issueLetsEncrypt($record->domain->domain)),
                Action::make('renew')
                    ->label(__('Renew'))
                    ->icon('heroicon-o-arrow-path')
                    ->color('info')
                    ->visible(fn (SslCertificate $record) => $record->service === 'web' && $record->type === 'lets_encrypt' && $record->status === 'active')
                    ->requiresConfirmation()
                    ->modalHeading(__('Renew SSL Certificate'))
                    ->modalDescription(fn (SslCertificate $record) => __("Renew the Let's Encrypt certificate for :domain? This will extend the certificate validity.", ['domain' => $record->domain->domain]))
                    ->modalIcon('heroicon-o-arrow-path')
                    ->modalIconColor('info')
                    ->modalSubmitActionLabel(__('Renew Certificate'))
                    ->action(fn (SslCertificate $record) => $this->renewCertificate($record->domain->domain)),
                Action::make('selfSigned')
                    ->label(__('Self-Signed'))
                    ->icon('heroicon-o-exclamation-triangle')
                    ->color('warning')
                    ->visible(fn (SslCertificate $record) => $record->service === 'web' && in_array($record->status, ['pending', 'failed']))
                    ->requiresConfirmation()
                    ->modalHeading(__('Generate Self-Signed Certificate'))
                    ->modalDescription(fn (SslCertificate $record) => __('Generate a self-signed certificate for :domain? Note: Browsers will show a security warning for self-signed certificates.', ['domain' => $record->domain->domain]))
                    ->modalIcon('heroicon-o-exclamation-triangle')
                    ->modalIconColor('warning')
                    ->modalSubmitActionLabel(__('Generate Certificate'))
                    ->action(fn (SslCertificate $record) => $this->generateSelfSigned($record->domain->domain)),
                Action::make('check')
                    ->label(__('Check'))
                    ->icon('heroicon-o-magnifying-glass')
                    ->color('gray')
                    ->visible(fn (SslCertificate $record) => $record->service === 'web')
                    ->action(fn (SslCertificate $record) => $this->checkCertificate($record->domain->domain)),
            ])
            ->emptyStateHeading(__('No SSL certificates'))
            ->emptyStateDescription(__('SSL certificates will appear here once issued for your domains'))
            ->emptyStateIcon('heroicon-o-lock-closed')
            ->striped();
    }

    public function issueLetsEncrypt(string $domainName): void
    {
        try {
            $domain = Domain::where('domain', $domainName)
                ->where('user_id', Auth::id())
                ->firstOrFail();

            $service = app(SslManagementService::class);
            $cert = $service->issue($domain);

            if ($cert->status === 'failed') {
                Notification::make()
                    ->title(__('SSL Certificate Failed'))
                    ->body($cert->last_error ?? __('Unknown error'))
                    ->danger()
                    ->send();

                return;
            }

            $domain->update(['ssl_enabled' => true]);

            Notification::make()
                ->title(__('SSL Certificate Issued'))
                ->body(__("Let's Encrypt certificate has been issued for :domain", ['domain' => $domainName]))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function generateSelfSigned(string $domainName): void
    {
        try {
            $domain = Domain::where('domain', $domainName)
                ->where('user_id', Auth::id())
                ->firstOrFail();

            $service = app(SslManagementService::class);
            $cert = $service->generateSelfSigned($domain);

            if ($cert->status === 'failed') {
                Notification::make()
                    ->title(__('Certificate Generation Failed'))
                    ->body($cert->last_error ?? __('Unknown error'))
                    ->danger()
                    ->send();

                return;
            }

            $domain->update(['ssl_enabled' => true]);

            Notification::make()
                ->title(__('Self-Signed Certificate Generated'))
                ->body(__('Self-signed certificate created for :domain', ['domain' => $domainName]))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function renewCertificate(string $domainName): void
    {
        try {
            $domain = Domain::where('domain', $domainName)
                ->where('user_id', Auth::id())
                ->firstOrFail();

            $service = app(SslManagementService::class);
            $cert = $service->renew($domain);

            if ($cert->status === 'failed') {
                Notification::make()
                    ->title(__('Renewal Failed'))
                    ->body($cert->last_error ?? __('Unknown error'))
                    ->danger()
                    ->send();

                return;
            }

            Notification::make()
                ->title(__('Certificate Renewed'))
                ->body(__('SSL certificate has been renewed for :domain', ['domain' => $domainName]))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function checkCertificate(string $domainName): void
    {
        try {
            $domain = Domain::where('domain', $domainName)
                ->where('user_id', Auth::id())
                ->firstOrFail();

            $service = app(SslManagementService::class);
            $cert = $service->check($domain);

            if ($cert->status === 'active') {
                $domain->update(['ssl_enabled' => true]);

                Notification::make()
                    ->title(__('Certificate Checked'))
                    ->body(__('Certificate found: :issuer', ['issuer' => $cert->issuer ?? __('Unknown')]))
                    ->success()
                    ->send();
            } else {
                Notification::make()
                    ->title(__('Certificate Checked'))
                    ->body(__('No certificate found'))
                    ->success()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function toggleAutoRenew(string $domainName): void
    {
        try {
            $domain = Domain::where('domain', $domainName)
                ->where('user_id', Auth::id())
                ->firstOrFail();

            $ssl = $domain->sslCertificate;
            if ($ssl) {
                $ssl->update(['auto_renew' => ! $ssl->auto_renew]);

                Notification::make()
                    ->title(__('Auto-Renew Updated'))
                    ->body($ssl->auto_renew ? __('Auto-renewal enabled') : __('Auto-renewal disabled'))
                    ->success()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function installCustomCertificateAction(): Action
    {
        return Action::make('installCustomCertificate')
            ->label(__('Install Custom Certificate'))
            ->icon('heroicon-o-document-plus')
            ->modalHeading(__('Install Custom SSL Certificate'))
            ->modalDescription(__('Upload your own SSL certificate files to secure your domain with a custom certificate.'))
            ->modalIcon('heroicon-o-document-plus')
            ->modalIconColor('primary')
            ->modalSubmitActionLabel(__('Install Certificate'))
            ->modalWidth('lg')
            ->form([
                Select::make('domain')
                    ->label(__('Domain'))
                    ->options(function () {
                        return Domain::where('user_id', Auth::id())
                            ->pluck('domain', 'domain')
                            ->toArray();
                    })
                    ->required()
                    ->searchable()
                    ->helperText(__('Select the domain to install the certificate on')),
                Textarea::make('certificate')
                    ->label(__('Certificate (PEM format)'))
                    ->placeholder(__("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
                    ->rows(8)
                    ->required()
                    ->helperText(__('Paste your SSL certificate in PEM format')),
                Textarea::make('private_key')
                    ->label(__('Private Key (PEM format)'))
                    ->placeholder(__("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"))
                    ->rows(8)
                    ->required()
                    ->helperText(__('Paste your private key in PEM format. Keep this secure!')),
                Textarea::make('ca_bundle')
                    ->label(__('CA Bundle (optional)'))
                    ->placeholder(__("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
                    ->rows(6)
                    ->helperText(__('Paste the certificate authority chain if required by your certificate provider')),
            ])
            ->action(function (array $data): void {
                try {
                    $domain = Domain::where('domain', $data['domain'])
                        ->where('user_id', Auth::id())
                        ->firstOrFail();

                    $service = app(SslManagementService::class);
                    $cert = $service->installCustom(
                        $domain,
                        $data['certificate'],
                        $data['private_key'],
                        $data['ca_bundle'] ?? null,
                    );

                    if ($cert->status === 'failed') {
                        Notification::make()
                            ->title(__('Installation Failed'))
                            ->body($cert->last_error ?? __('Unknown error'))
                            ->danger()
                            ->send();

                        return;
                    }

                    $domain->update(['ssl_enabled' => true]);

                    Notification::make()
                        ->title(__('Certificate Installed'))
                        ->body(__('Custom SSL certificate installed for :domain', ['domain' => $data['domain']]))
                        ->success()
                        ->send();
                } catch (Exception $e) {
                    Notification::make()
                        ->title(__('Error'))
                        ->body(SafeError::message($e))
                        ->danger()
                        ->send();
                }
            });
    }

    protected function getHeaderActions(): array
    {
        return [
            $this->installCustomCertificateAction(),
        ];
    }
}
