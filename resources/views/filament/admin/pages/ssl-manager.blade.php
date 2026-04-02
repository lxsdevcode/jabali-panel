<x-filament-panels::page>
    @if($autoSslLog)
        <x-filament::section
            icon="heroicon-o-command-line"
            icon-color="success"
            collapsible
        >
            <x-slot name="heading">{{ __('SSL Check Log') }}</x-slot>
            <x-slot name="headerEnd">
                <x-filament::icon-button
                    wire:click="$set('autoSslLog', '')"
                    icon="heroicon-o-x-mark"
                    color="gray"
                    size="sm"
                    tooltip="{{ __('Clear Log') }}"
                />
            </x-slot>
            <div class="rounded-lg overflow-hidden">
                <pre class="bg-gray-950 p-4 font-mono fi-section-header-description leading-relaxed max-h-80 overflow-y-auto whitespace-pre-wrap">{{ $autoSslLog }}</pre>
            </div>
        </x-filament::section>
    @endif

    <x-filament::section>
        <x-slot name="heading">{{ __('SSL Certificates') }}</x-slot>

        <div class="divide-y divide-gray-200 dark:divide-white/10">
            @forelse ($this->userCerts as $user)
                <div x-data="{ open: false }" class="py-1">
                    {{-- User row --}}
                    <button
                        type="button"
                        x-on:click="open = !open"
                        class="flex w-full items-center justify-between gap-4 rounded-lg px-3 py-2.5 text-left transition hover:bg-gray-50 dark:hover:bg-white/5"
                    >
                        <div class="flex items-center gap-3">
                            <div x-bind:class="open ? 'rotate-90' : ''" class="transition-transform duration-200">
                                <x-heroicon-m-chevron-right class="h-4 w-4 text-gray-400" />
                            </div>
                            <div>
                                <x-heroicon-o-user class="inline h-4 w-4 text-gray-400" />
                                <span class="text-sm font-semibold text-gray-950 dark:text-white">{{ $user->username }}</span>
                            </div>
                        </div>
                        <div class="flex items-center gap-2">
                            <x-filament::badge size="sm" color="gray">
                                {{ $user->domains->sum(fn ($d) => $d->sslCertificates->count()) }} {{ __('certs') }}
                            </x-filament::badge>
                            @php
                                $allCerts = $user->domains->flatMap->sslCertificates;
                                $allActive = $allCerts->every(fn ($c) => $c->status === 'active');
                                $hasFailed = $allCerts->contains(fn ($c) => $c->status === 'failed');
                            @endphp
                            @if ($allActive && $allCerts->isNotEmpty())
                                <x-filament::badge size="sm" color="success">{{ __('Active') }}</x-filament::badge>
                            @elseif ($hasFailed)
                                <x-filament::badge size="sm" color="danger">{{ __('Failed') }}</x-filament::badge>
                            @else
                                <x-filament::badge size="sm" color="warning">{{ __('Pending') }}</x-filament::badge>
                            @endif
                        </div>
                    </button>

                    {{-- Domains grouped by root domain --}}
                    <div x-show="open" x-collapse x-cloak class="mt-1 mb-2 ml-7 mr-2">
                        @foreach ($user->groupedDomains as $rootDomain => $domains)
                            <div class="mb-3">
                                <p class="mb-1 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                                    {{ $rootDomain }}
                                </p>
                                <table class="w-full text-sm">
                                    <thead>
                                        <tr class="border-b border-gray-200 dark:border-white/10">
                                            <th class="px-3 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ __('Hostname') }}</th>
                                            <th class="px-3 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ __('Service') }}</th>
                                            <th class="px-3 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ __('Type') }}</th>
                                            <th class="px-3 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ __('Status') }}</th>
                                            <th class="px-3 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ __('Expires') }}</th>
                                            <th class="px-3 py-1.5 text-right text-xs font-medium text-gray-500 dark:text-gray-400">{{ __('Actions') }}</th>
                                        </tr>
                                    </thead>
                                    <tbody class="divide-y divide-gray-100 dark:divide-white/5">
                                        @foreach ($domains as $domain)
                                        @foreach ($domain->sslCertificates->sortBy('service') as $cert)
                                            <tr>
                                                <td class="px-3 py-2 text-gray-950 dark:text-white">
                                                    {{ $cert->service === 'mail' ? 'mail.' . $domain->domain : $domain->domain }}
                                                </td>
                                                <td class="px-3 py-2">
                                                    <x-filament::badge size="sm" :color="$cert->service === 'web' ? 'info' : 'warning'">
                                                        {{ $cert->service === 'web' ? __('HTTPS') : __('Mail') }}
                                                    </x-filament::badge>
                                                </td>
                                                <td class="px-3 py-2">
                                                    <x-filament::badge size="sm" color="gray">
                                                        {{ $cert->type ? ucfirst(str_replace('_', ' ', $cert->type)) : __('None') }}
                                                    </x-filament::badge>
                                                </td>
                                                <td class="px-3 py-2">
                                                    <x-filament::badge size="sm" :color="match($cert->status) { 'active' => 'success', 'expired' => 'danger', 'failed' => 'danger', default => 'gray' }">
                                                        {{ ucfirst($cert->status ?? 'unknown') }}
                                                    </x-filament::badge>
                                                </td>
                                                <td class="px-3 py-2 text-xs text-gray-600 dark:text-gray-400">
                                                    @if ($cert->expires_at)
                                                        {{ $cert->expires_at->format('M d, Y') }}
                                                        <span class="text-xs {{ $cert->days_until_expiry <= 7 ? 'text-danger-500' : ($cert->days_until_expiry <= 30 ? 'text-warning-500' : 'text-gray-400') }}">
                                                            ({{ $cert->days_until_expiry }}d)
                                                        </span>
                                                    @else
                                                        -
                                                    @endif
                                                </td>
                                                <td class="px-3 py-2 text-right">
                                                    <div class="flex items-center justify-end gap-1">
                                                        @if ($cert->type === 'lets_encrypt' && $cert->status === 'active')
                                                            <x-filament::icon-button
                                                                wire:click="renewSslForDomain({{ $cert->domain_id }}, '{{ $cert->service }}')"
                                                                icon="heroicon-o-arrow-path"
                                                                color="primary"
                                                                size="sm"
                                                                tooltip="{{ __('Renew') }}"
                                                            />
                                                        @endif
                                                        @if (in_array($cert->type, ['self_signed', 'none', '']) || in_array($cert->status, ['pending', 'failed']))
                                                            <x-filament::icon-button
                                                                wire:click="{{ $cert->service === 'mail' ? 'issueMailSslForDomain' : 'issueSslForDomain' }}({{ $cert->domain_id }})"
                                                                icon="heroicon-o-check-circle"
                                                                color="success"
                                                                size="sm"
                                                                tooltip="{{ __('Issue') }}"
                                                            />
                                                        @endif
                                                        <x-filament::icon-button
                                                            wire:click="checkSslForDomain({{ $cert->domain_id }})"
                                                            icon="heroicon-o-magnifying-glass"
                                                            color="gray"
                                                            size="sm"
                                                            tooltip="{{ __('Check') }}"
                                                        />
                                                    </div>
                                                </td>
                                            </tr>
                                        @endforeach
                                        @endforeach
                                    </tbody>
                                </table>
                            </div>
                        @endforeach
                    </div>
                </div>
            @empty
                <div class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                    {{ __('No SSL certificates found. Run SSL Check to scan your domains.') }}
                </div>
            @endforelse
        </div>
    </x-filament::section>
</x-filament-panels::page>
