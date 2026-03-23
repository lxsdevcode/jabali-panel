<div>
    {{-- Breadcrumbs --}}
    <nav class="flex items-center gap-1 text-sm mb-3">
        @foreach($breadcrumbs as $path => $label)
            @if(!$loop->last)
                <button wire:click="navigateTo('{{ $path }}')" class="fi-link fi-link-size-sm text-primary-600 dark:text-primary-400 hover:underline">
                    @if($loop->first)
                        <x-filament::icon icon="heroicon-o-home" class="inline h-4 w-4" />
                    @endif
                    {{ $label }}
                </button>
                <x-filament::icon icon="heroicon-o-chevron-right" class="h-3 w-3 text-gray-400" />
            @else
                <span class="text-gray-500 dark:text-gray-400">{{ $label }}</span>
            @endif
        @endforeach
    </nav>

    @if(count($this->selectedFiles) > 0)
        <x-filament::section compact class="mb-3">
            <span class="text-sm text-success-600 dark:text-success-400 font-medium">
                {{ __(':count file(s)/folder(s) selected for restore', ['count' => count($this->selectedFiles)]) }}
            </span>
        </x-filament::section>
    @endif

    {{-- Table --}}
    {{ $this->table }}

    <x-filament-actions::modals />
</div>
