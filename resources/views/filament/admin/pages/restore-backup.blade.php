<x-filament-panels::page>
    @php
        $items = $this->loadDirectory();
    @endphp

    <x-filament::section>
        <x-slot name="heading">
            <div class="flex items-center gap-2">
                <x-heroicon-o-folder-open class="h-5 w-5" />
                <span>{{ $this->currentPath ?: '/' }}</span>
            </div>
        </x-slot>

        <div class="divide-y divide-gray-200 dark:divide-white/10">
            @forelse($items as $item)
                @if($item['is_dir'] ?? false)
                    <button
                        wire:click="navigateTo('{{ $item['path'] }}')"
                        class="flex w-full items-center gap-3 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-white/5 transition"
                    >
                        @if(($item['is_parent'] ?? false))
                            <x-heroicon-o-arrow-up class="h-4 w-4 text-gray-400 shrink-0" />
                        @else
                            <x-heroicon-o-folder class="h-4 w-4 text-yellow-500 shrink-0" />
                        @endif
                        <span class="font-medium text-gray-950 dark:text-white">{{ $item['name'] }}</span>
                    </button>
                @else
                    <div class="flex items-center gap-3 px-3 py-2 text-sm">
                        <x-heroicon-o-document class="h-4 w-4 text-gray-400 shrink-0" />
                        <span class="text-gray-700 dark:text-gray-300 flex-1">{{ $item['name'] }}</span>
                        @if($item['size'] ?? null)
                            <span class="text-xs text-gray-500">{{ \App\Support\Formatter::bytes($item['size']) }}</span>
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

    <x-filament-actions::modals />
</x-filament-panels::page>
