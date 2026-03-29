@if(empty($this->contents))
    <x-filament::loading-indicator class="h-8 w-8 mx-auto" />
@else
    <div class="grid gap-4 md:grid-cols-3">
        {{-- Files --}}
        <div class="fi-section rounded-xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10 p-4">
            <div class="flex items-center gap-2 mb-2">
                <x-heroicon-o-folder class="h-5 w-5 text-yellow-500" />
                <span class="text-sm font-semibold text-gray-950 dark:text-white">{{ __('Files') }}</span>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-2">{{ count($this->contents['domains'] ?? []) }} {{ __('domain(s)') }}</p>
            <div class="flex flex-wrap gap-1">
                @foreach(($this->contents['domains'] ?? []) as $domain)
                    <x-filament::badge color="info">{{ $domain }}</x-filament::badge>
                @endforeach
            </div>
        </div>

        {{-- Databases --}}
        <div class="fi-section rounded-xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10 p-4">
            <div class="flex items-center gap-2 mb-2">
                <x-heroicon-o-circle-stack class="h-5 w-5 text-blue-500" />
                <span class="text-sm font-semibold text-gray-950 dark:text-white">{{ __('Databases') }}</span>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-2">{{ count($this->contents['databases'] ?? []) }} {{ __('database(s)') }}</p>
            <div class="flex flex-wrap gap-1">
                @foreach(($this->contents['databases'] ?? []) as $db)
                    <x-filament::badge color="warning">{{ $db }}</x-filament::badge>
                @endforeach
            </div>
            @if(!empty($this->contents['db_users'] ?? []))
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">{{ count($this->contents['db_users']) }} {{ __('MySQL user(s)') }}</p>
                <div class="flex flex-wrap gap-1 mt-1">
                    @foreach($this->contents['db_users'] as $dbUser)
                        <x-filament::badge color="gray">{{ $dbUser }}</x-filament::badge>
                    @endforeach
                </div>
            @endif
        </div>

        {{-- Email --}}
        <div class="fi-section rounded-xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10 p-4">
            <div class="flex items-center gap-2 mb-2">
                <x-heroicon-o-envelope class="h-5 w-5 text-green-500" />
                <span class="text-sm font-semibold text-gray-950 dark:text-white">{{ __('Email') }}</span>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-2">{{ count($this->contents['mailboxes'] ?? []) }} {{ __('mailbox(es)') }}</p>
            <div class="flex flex-wrap gap-1">
                @foreach(($this->contents['mailboxes'] ?? []) as $mbox)
                    <x-filament::badge color="gray">{{ $mbox }}</x-filament::badge>
                @endforeach
            </div>
        </div>
    </div>
@endif
