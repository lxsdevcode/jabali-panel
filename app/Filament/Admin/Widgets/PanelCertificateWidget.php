<?php

declare(strict_types=1);

namespace App\Filament\Admin\Widgets;

use App\Models\PanelCertificate;
use Filament\Widgets\Widget;

class PanelCertificateWidget extends Widget
{
    protected ?string $pollingInterval = '30s';

    protected int|string|array $columnSpan = 'full';

    protected string $view = 'filament.admin.widgets.panel-certificate';

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
