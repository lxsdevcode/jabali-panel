<x-filament-panels::page>
    @php
        $items = $this->loadDirectory();
    @endphp

    <x-filament::section>
        <x-slot name="heading">
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                    <x-heroicon-o-folder-open class="h-5 w-5" />
                    <span>{{ $this->currentPath ?: '/' }}</span>
                </div>
                @if(count($this->selectedPaths) > 0)
                    <x-filament::badge color="primary">
                        {{ count($this->selectedPaths) }} {{ __('selected') }}
                    </x-filament::badge>
                @endif
            </div>
        </x-slot>

        <div class="divide-y divide-gray-200 dark:divide-white/10">
            @forelse($items as $item)
                @if(($item['is_parent'] ?? false))
                    <button
                        wire:click="navigateTo('{{ $item['path'] }}')"
                        class="flex w-full items-center gap-3 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-white/5 transition"
                    >
                        <x-heroicon-o-arrow-up class="h-4 w-4 text-gray-400 shrink-0" />
                        <span class="font-medium text-gray-950 dark:text-white">..</span>
                    </button>
                @else
                    <div class="flex items-center gap-3 px-3 py-2 text-sm hover:bg-gray-50 dark:hover:bg-white/5 transition">
                        <input
                            type="checkbox"
                            value="{{ $item['path'] }}"
                            wire:model.live="selectedPaths"
                            class="rounded border-gray-300 text-primary-600 shadow-sm focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700"
                        />

                        @if($item['is_dir'] ?? false)
                            <button
                                wire:click="navigateTo('{{ $item['path'] }}')"
                                class="flex items-center gap-2 flex-1 text-left"
                            >
                                <x-heroicon-o-folder class="h-4 w-4 text-yellow-500 shrink-0" />
                                <span class="font-medium text-gray-950 dark:text-white">{{ $item['name'] }}</span>
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
                <div class="px-3 py-8 text-center text-sm text-gray-500">
                    {{ __('Empty directory') }}
                </div>
            @endforelse
        </div>
    </x-filament::section>

    @if(count($this->selectedPaths) > 0)
    <div class="mt-4">
        <x-filament::section compact>
            <div class="flex items-center justify-between">
                <div class="text-sm text-gray-700 dark:text-gray-300">
                    <span class="font-medium">{{ count($this->selectedPaths) }}</span> {{ __('item(s) selected') }}:
                    <span class="text-gray-500">{{ implode(', ', array_map('basename', $this->selectedPaths)) }}</span>
                </div>
                <div class="flex gap-2">
                    <x-filament::button color="gray" size="sm" wire:click="$set('selectedPaths', [])">
                        {{ __('Clear') }}
                    </x-filament::button>
                </div>
            </div>
        </x-filament::section>
    </div>
    @endif

    <x-filament-actions::modals />
</x-filament-panels::page>
