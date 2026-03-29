<x-filament::tabs>
    <x-filament::tabs.item :active="$this->activeSection === 'files'" wire:click="selectSectionAndRefresh('files')" icon="heroicon-o-folder">
        {{ __('Files') }}
    </x-filament::tabs.item>
    <x-filament::tabs.item :active="$this->activeSection === 'databases'" wire:click="selectSectionAndRefresh('databases')" icon="heroicon-o-circle-stack">
        {{ __('Databases') }}
    </x-filament::tabs.item>
    <x-filament::tabs.item :active="$this->activeSection === 'mailboxes'" wire:click="selectSectionAndRefresh('mailboxes')" icon="heroicon-o-envelope">
        {{ __('Email') }}
    </x-filament::tabs.item>
</x-filament::tabs>

@if($this->activeSection === 'files')
    <x-filament::section>
        <x-slot name="heading">
            <div class="flex items-center gap-2">
                <x-heroicon-o-folder-open class="h-5 w-5" />
                <span>{{ $this->currentPath ?: '/' }}</span>
                @if(count($this->selectedPaths) > 0)
                    <x-filament::badge color="primary">{{ count($this->selectedPaths) }} {{ __('selected') }}</x-filament::badge>
                @endif
            </div>
        </x-slot>

        <div class="divide-y divide-gray-200 dark:divide-white/10">
            @forelse($this->directoryItems as $idx => $item)
                @if(($item['is_parent'] ?? false))
                    <button wire:key="dir-parent" x-on:click="$wire.navigateTo({{ \Illuminate\Support\Js::from($item['path']) }})" class="flex w-full items-center gap-3 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-white/5 transition">
                        <x-heroicon-o-arrow-up class="h-4 w-4 text-gray-400 shrink-0" />
                        <span class="text-sm font-medium text-gray-950 dark:text-white">..</span>
                    </button>
                @else
                    <div wire:key="dir-item-{{ $idx }}" class="flex items-center gap-3 px-3 py-2 text-sm hover:bg-gray-50 dark:hover:bg-white/5">
                        <x-filament::input.checkbox value="{{ $item['path'] }}" wire:model="selectedPaths" />
                        @if($item['is_dir'] ?? false)
                            <button x-on:click="$wire.navigateTo({{ \Illuminate\Support\Js::from($item['path']) }})" class="flex items-center gap-2 flex-1 text-left">
                                <x-heroicon-o-folder class="h-4 w-4 text-yellow-500 shrink-0" />
                                <span class="text-sm font-medium text-gray-950 dark:text-white">{{ $item['name'] }}</span>
                            </button>
                        @else
                            <x-heroicon-o-document class="h-4 w-4 text-gray-400 shrink-0" />
                            <span class="text-gray-700 dark:text-gray-300 flex-1">{{ $item['name'] }}</span>
                            @if($item['size'] ?? null)
                                <span class="text-xs text-gray-500">{{ \App\Support\Formatter::bytes($item['size']) }}</span>
                            @endif
                        @endif
                    </div>
                @endif
            @empty
                <div class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ __('Empty directory') }}</div>
            @endforelse
        </div>
    </x-filament::section>

@elseif($this->activeSection === 'databases')
    <x-filament::section :heading="__('Select Databases')">
        @forelse($this->contents['databases'] ?? [] as $db)
            <label wire:key="db-{{ $db }}" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-50 dark:hover:bg-white/5 cursor-pointer">
                <x-filament::input.checkbox value="{{ $db }}" wire:model="selectedDatabases" />
                <x-heroicon-o-circle-stack class="h-4 w-4 text-blue-500" />
                <span class="text-sm font-medium text-gray-950 dark:text-white">{{ $db }}</span>
            </label>
        @empty
            <p class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">{{ __('No databases in this backup') }}</p>
        @endforelse
        @if($this->contents['has_db_users'] ?? false)
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-3 px-3">{{ __('MySQL users and grants will be restored automatically with the selected databases.') }}</p>
        @endif
    </x-filament::section>

@elseif($this->activeSection === 'mailboxes')
    <x-filament::section :heading="__('Select Mailboxes')">
        @forelse($this->contents['mailboxes'] ?? [] as $mbox)
            <label wire:key="mbox-{{ $mbox }}" class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-50 dark:hover:bg-white/5 cursor-pointer">
                <x-filament::input.checkbox value="{{ $mbox }}" wire:model="selectedMailboxes" />
                <x-heroicon-o-envelope class="h-4 w-4 text-green-500" />
                <span class="text-sm font-medium text-gray-950 dark:text-white">{{ $mbox }}</span>
            </label>
        @empty
            <p class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">{{ __('No mailboxes in this backup') }}</p>
        @endforelse
    </x-filament::section>
@endif
