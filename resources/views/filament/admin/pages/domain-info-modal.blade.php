<div class="space-y-4">
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
            <p class="text-sm text-gray-900 dark:text-white">
                @switch($domain->dns_status)
                    @case('points_here') {{ __('Points Here') }} @break
                    @case('cloudflare') {{ __('Cloudflare') }} @break
                    @case('external') {{ __('External') }} @break
                    @case('dns_missing') {{ __('DNS Missing') }} @break
                    @case('dns_error') {{ __('DNS Error') }} @break
                    @default {{ __('Unchecked') }}
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

    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ __('WHOIS Information') }}</h3>
    <div class="grid grid-cols-2 gap-4">
        <div>
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Registration Status') }}</p>
            <p class="text-sm text-gray-900 dark:text-white">
                @switch($domain->whois_status)
                    @case('registered') {{ __('Registered') }} @break
                    @case('expired') {{ __('Expired') }} @break
                    @case('unregistered') {{ __('Unregistered') }} @break
                    @case('whois_error') {{ __('Error') }} @break
                    @default {{ __('Unchecked') }}
                @endswitch
            </p>
        </div>
        <div>
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Expiry Date') }}</p>
            <p class="text-sm text-gray-900 dark:text-white">{{ $domain->whois_expiry?->format('Y-m-d') ?? __('N/A') }}</p>
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
