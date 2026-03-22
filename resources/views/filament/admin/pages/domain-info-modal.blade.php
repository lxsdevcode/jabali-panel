<div x-data="{ tab: 'general' }">
    {{-- Tab buttons --}}
    <div class="flex gap-2 border-b border-gray-200 dark:border-white/10 mb-4">
        <button type="button"
            @click="tab = 'general'"
            :class="tab === 'general' ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'"
            class="px-4 py-2 text-sm font-medium border-b-2 -mb-px">
            {{ __('General') }}
        </button>
        <button type="button"
            @click="tab = 'whois'"
            :class="tab === 'whois' ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'"
            class="px-4 py-2 text-sm font-medium border-b-2 -mb-px">
            {{ __('WHOIS') }}
        </button>
    </div>

    {{-- General tab --}}
    <div x-show="tab === 'general'" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Domain') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $domain->domain }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Owner') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $domain->user?->username ?? __('N/A') }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Document Root') }}</p>
                <p class="text-sm font-mono text-gray-900 dark:text-white">{{ $domain->document_root ?? __('N/A') }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Active') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $domain->is_active ? __('Yes') : __('No') }}</p>
            </div>
        </div>

        <hr class="border-gray-200 dark:border-white/10" />

        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ __('DNS Information') }}</h3>
        <div class="grid grid-cols-2 gap-4">
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('DNS Status') }}</p>
                <p class="text-sm">
                    @switch($domain->dns_status)
                        @case('points_here') <span class="text-success-600 dark:text-success-400">{{ __('Points Here') }}</span> @break
                        @case('cloudflare') <span class="text-info-600 dark:text-info-400">{{ __('Cloudflare') }}</span> @break
                        @case('external') <span class="text-warning-600 dark:text-warning-400">{{ __('External') }}</span> @break
                        @case('dns_missing') <span class="text-gray-500">{{ __('DNS Missing') }}</span> @break
                        @case('dns_error') <span class="text-danger-600 dark:text-danger-400">{{ __('DNS Error') }}</span> @break
                        @default <span class="text-gray-400">{{ __('Unchecked') }}</span>
                    @endswitch
                </p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Resolved IP') }}</p>
                <p class="text-sm font-mono text-gray-900 dark:text-white">{{ $domain->dns_resolved_ip ?? __('N/A') }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Server IP') }}</p>
                <p class="text-sm font-mono text-gray-900 dark:text-white">{{ \App\Support\ServerFacts::serverIp('N/A') }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Last DNS Check') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $domain->dns_checked_at?->diffForHumans() ?? __('Never') }}</p>
            </div>
        </div>

        <hr class="border-gray-200 dark:border-white/10" />

        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ __('SSL Information') }}</h3>
        <div class="grid grid-cols-2 gap-4">
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('SSL Enabled') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $domain->ssl_enabled ? __('Yes') : __('No') }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('SSL Status') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $domain->ssl_status }}</p>
            </div>
        </div>
    </div>

    {{-- WHOIS tab --}}
    <div x-show="tab === 'whois'" x-cloak class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Registration Status') }}</p>
                <p class="text-sm">
                    @switch($whoisData['status'] ?? null)
                        @case('registered') <span class="text-success-600 dark:text-success-400">{{ __('Registered') }}</span> @break
                        @case('expired') <span class="text-danger-600 dark:text-danger-400">{{ __('Expired') }}</span> @break
                        @case('unregistered') <span class="text-gray-500">{{ __('Unregistered') }}</span> @break
                        @case('whois_error') <span class="text-warning-600 dark:text-warning-400">{{ __('Error') }}</span> @break
                        @default <span class="text-gray-400">{{ __('Unknown') }}</span>
                    @endswitch
                </p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Expiry Date') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $whoisData['expiry'] ?? __('N/A') }}</p>
            </div>
            <div>
                <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Registrar') }}</p>
                <p class="text-sm text-gray-900 dark:text-white">{{ $whoisData['registrar'] ?? __('N/A') }}</p>
            </div>
        </div>

        @if(!empty($whoisData['raw']))
            <hr class="border-gray-200 dark:border-white/10" />

            <div x-data="{ showRaw: false }">
                <button type="button" @click="showRaw = !showRaw" class="text-sm text-primary-500 hover:underline">
                    <span x-show="!showRaw">{{ __('Show Raw WHOIS Output') }}</span>
                    <span x-show="showRaw" x-cloak>{{ __('Hide Raw WHOIS Output') }}</span>
                </button>

                <div x-show="showRaw" x-cloak class="mt-2">
                    <pre class="max-h-80 overflow-auto rounded-lg bg-gray-50 p-3 text-xs font-mono text-gray-700 dark:bg-gray-900 dark:text-gray-300">{{ $whoisData['raw'] }}</pre>
                </div>
            </div>
        @endif
    </div>
</div>
