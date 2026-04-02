<?php

declare(strict_types=1);

namespace App\Filament\Admin\Widgets;

use App\Models\PanelCertificate;
use App\Services\Agent\AgentClient;
use App\Support\SafeError;
use Exception;
use Filament\Notifications\Notification;
use Filament\Widgets\Widget;

class PanelCertificateWidget extends Widget
{
    protected ?string $pollingInterval = '30s';

    protected int|string|array $columnSpan = 'full';

    protected string $view = 'filament.admin.widgets.panel-certificate';

    public function issuePanelCert(): void
    {
        try {
            $agent = app(AgentClient::class);
            $result = $agent->sslPanelIssue();

            if ($result['success'] ?? false) {
                // Re-sync cert info
                \Artisan::call('jabali:panel-cert-sync');

                Notification::make()
                    ->title(__('Panel certificate issued'))
                    ->body($result['message'] ?? __('Let\'s Encrypt certificate installed successfully.'))
                    ->success()
                    ->send();
            } else {
                Notification::make()
                    ->title(__('Certificate issuance failed'))
                    ->body($result['error'] ?? __('Unknown error'))
                    ->danger()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Certificate issuance failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    protected function getCertificate(): ?PanelCertificate
    {
        return PanelCertificate::first();
    }

    protected function getCertData(): array
    {
        $cert = $this->getCertificate();

        if (! $cert) {
            return [
                'hostname' => config('jabali.panel.hostname') ?: __('Not configured'),
                'status' => 'pending',
                'status_label' => __('No Certificate'),
                'status_color' => 'gray',
                'issuer' => '-',
                'expires_at' => null,
                'days_remaining' => null,
                'last_renewal_at' => null,
                'last_renewal_result' => null,
                'last_error' => null,
                'is_self_signed' => false,
            ];
        }

        return [
            'hostname' => $cert->hostname,
            'status' => $cert->status,
            'status_label' => $cert->status_label,
            'status_color' => $cert->status_color,
            'issuer' => $cert->issuer ?? '-',
            'expires_at' => $cert->expires_at?->format('M j, Y H:i'),
            'days_remaining' => $cert->days_until_expiry,
            'last_renewal_at' => $cert->last_renewal_at?->diffForHumans(),
            'last_renewal_result' => $cert->last_renewal_result,
            'last_error' => $cert->last_error,
            'is_self_signed' => $cert->isSelfSigned(),
        ];
    }
}
