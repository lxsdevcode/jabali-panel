@if($contents['error'])
    <div class="rounded-lg bg-red-50 p-4 dark:bg-red-950/20">
        <p class="text-sm text-red-700 dark:text-red-400">{{ $contents['error'] }}</p>
    </div>
@else
    <div class="space-y-4">
        <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ __(':count files in snapshot', ['count' => $contents['total_files']]) }}
        </p>

        @if(!empty($contents['domains']))
        <div>
            <h4 class="text-sm font-semibold text-gray-950 dark:text-white mb-2">
                <x-heroicon-o-globe-alt class="inline h-4 w-4" />
                {{ __('Domains') }} ({{ count($contents['domains']) }})
            </h4>
            <div class="flex flex-wrap gap-2">
                @foreach($contents['domains'] as $domain)
                    <x-filament::badge color="info">{{ $domain }}</x-filament::badge>
                @endforeach
            </div>
        </div>
        @endif

        @if(!empty($contents['databases']))
        <div>
            <h4 class="text-sm font-semibold text-gray-950 dark:text-white mb-2">
                <x-heroicon-o-circle-stack class="inline h-4 w-4" />
                {{ __('Databases') }} ({{ count($contents['databases']) }})
            </h4>
            <div class="flex flex-wrap gap-2">
                @foreach($contents['databases'] as $db)
                    <x-filament::badge color="warning">{{ $db }}</x-filament::badge>
                @endforeach
            </div>
        </div>
        @endif

        @if(!empty($contents['mailboxes']))
        <div>
            <h4 class="text-sm font-semibold text-gray-950 dark:text-white mb-2">
                <x-heroicon-o-envelope class="inline h-4 w-4" />
                {{ __('Mailboxes') }} ({{ count($contents['mailboxes']) }})
            </h4>
            <div class="flex flex-wrap gap-2">
                @foreach($contents['mailboxes'] as $mbox)
                    <x-filament::badge color="gray">{{ $mbox }}</x-filament::badge>
                @endforeach
            </div>
        </div>
        @endif

        @if(!empty($contents['dns_zones']))
        <div>
            <h4 class="text-sm font-semibold text-gray-950 dark:text-white mb-2">
                <x-heroicon-o-server-stack class="inline h-4 w-4" />
                {{ __('DNS Zones') }} ({{ count($contents['dns_zones']) }})
            </h4>
            <div class="flex flex-wrap gap-2">
                @foreach($contents['dns_zones'] as $zone)
                    <x-filament::badge color="gray">{{ $zone }}</x-filament::badge>
                @endforeach
            </div>
        </div>
        @endif

        @if(!empty($contents['ssl_certs']))
        <div>
            <h4 class="text-sm font-semibold text-gray-950 dark:text-white mb-2">
                <x-heroicon-o-shield-check class="inline h-4 w-4" />
                {{ __('SSL Certificates') }} ({{ count($contents['ssl_certs']) }})
            </h4>
            <div class="flex flex-wrap gap-2">
                @foreach($contents['ssl_certs'] as $cert)
                    <x-filament::badge color="success">{{ $cert }}</x-filament::badge>
                @endforeach
            </div>
        </div>
        @endif

        @if(empty($contents['domains']) && empty($contents['databases']) && empty($contents['mailboxes']))
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ __('No categorized content found in this snapshot.') }}</p>
        @endif
    </div>
@endif
