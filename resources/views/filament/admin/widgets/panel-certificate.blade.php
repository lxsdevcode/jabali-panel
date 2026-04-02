<x-filament-widgets::widget>
    @php $cert = $this->getCertData(); @endphp
    <x-filament::section>
        <x-slot name="heading">
            <div class="flex items-center gap-2">
                <x-heroicon-m-server-stack class="h-5 w-5 text-gray-400" />
                <span>{{ __('Panel Certificate') }}</span>
                <x-filament::badge :color="$cert['status_color']">
                    {{ $cert['status_label'] }}
                </x-filament::badge>
            </div>
        </x-slot>

        <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Hostname') }}</p>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">{{ $cert['hostname'] }}</p>
            </div>

            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Issuer') }}</p>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">{{ $cert['issuer'] }}</p>
            </div>

            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Expires') }}</p>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">
                    @if($cert['expires_at'])
                        {{ $cert['expires_at'] }}
                        @if($cert['days_remaining'] !== null)
                            <span class="text-xs {{ $cert['days_remaining'] <= 7 ? 'text-red-500' : ($cert['days_remaining'] <= 30 ? 'text-yellow-500' : 'text-green-500') }}">
                                ({{ $cert['days_remaining'] }}d)
                            </span>
                        @endif
                    @else
                        -
                    @endif
                </p>
            </div>

            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Last Renewal') }}</p>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">
                    @if($cert['last_renewal_at'])
                        {{ $cert['last_renewal_at'] }}
                        @if($cert['last_renewal_result'] === 'success')
                            <x-heroicon-m-check-circle class="inline h-4 w-4 text-green-500" />
                        @elseif($cert['last_renewal_result'] === 'failed')
                            <x-heroicon-m-x-circle class="inline h-4 w-4 text-red-500" />
                        @endif
                    @else
                        -
                    @endif
                </p>
            </div>
        </div>

        @if($cert['last_error'])
            <div class="mt-3 rounded-lg bg-red-50 p-3 dark:bg-red-950/20">
                <p class="text-sm text-red-700 dark:text-red-400">{{ $cert['last_error'] }}</p>
            </div>
        @endif

        @if($cert['is_self_signed'] || $cert['status'] === 'pending')
            <div class="mt-3 flex items-center justify-between rounded-lg bg-yellow-50 p-3 dark:bg-yellow-950/20">
                <p class="text-sm text-yellow-700 dark:text-yellow-400">
                    {{ __('Using self-signed certificate. Click "Issue Certificate" to request a Let\'s Encrypt certificate.') }}
                </p>
                <x-filament::button
                    size="sm"
                    wire:click="issuePanelCert"
                    wire:loading.attr="disabled"
                    icon="heroicon-m-shield-check"
                >
                    {{ __('Issue Certificate') }}
                </x-filament::button>
            </div>
        @endif
    </x-filament::section>
</x-filament-widgets::widget>
