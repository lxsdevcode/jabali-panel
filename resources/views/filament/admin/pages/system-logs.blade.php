<x-filament-panels::page>
    @php
        $tabs = \App\Filament\Admin\Pages\SystemLogs::getLogTabs();
        $currentTab = $tabs[$this->activeTab] ?? [];
        $logs = $currentTab['logs'] ?? [];
    @endphp

    {{-- Tabs --}}
    <x-filament::tabs>
        @foreach($tabs as $key => $tab)
            <x-filament::tabs.item
                :active="$this->activeTab === $key"
                :icon="$tab['icon']"
                wire:click="setTab('{{ $key }}')"
            >
                {{ __($tab['label']) }}
            </x-filament::tabs.item>
        @endforeach
    </x-filament::tabs>

    {{-- Controls --}}
    <div class="flex flex-wrap items-center gap-3">
        {{-- Log file selector --}}
        <select
            wire:change="setLog($event.target.value)"
            class="fi-select-input rounded-lg border-gray-300 bg-white text-sm shadow-sm dark:border-white/10 dark:bg-white/5 dark:text-white"
        >
            @foreach($logs as $logKey => $logLabel)
                <option value="{{ $logKey }}" @selected($this->selectedLog === $logKey)>{{ $logLabel }}</option>
            @endforeach
        </select>

        {{-- Line count --}}
        <select
            wire:change="setLineCount(parseInt($event.target.value))"
            class="fi-select-input rounded-lg border-gray-300 bg-white text-sm shadow-sm dark:border-white/10 dark:bg-white/5 dark:text-white"
        >
            @foreach([100, 200, 500, 1000] as $count)
                <option value="{{ $count }}" @selected($this->lineCount === $count)>{{ $count }} {{ __('lines') }}</option>
            @endforeach
        </select>

        {{-- Refresh --}}
        <button
            wire:click="loadLog"
            class="fi-btn fi-btn-size-sm inline-flex items-center gap-1 rounded-lg bg-gray-100 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10"
        >
            <x-heroicon-o-arrow-path class="h-4 w-4" wire:loading.class="animate-spin" wire:target="loadLog" />
            {{ __('Refresh') }}
        </button>

        {{-- File info --}}
        <div class="ml-auto flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
            @if($this->fileSize > 0)
                <span>{{ $this->formatFileSize($this->fileSize) }}</span>
            @endif
            @if($this->lastModified)
                <span>{{ __('Modified') }}: {{ $this->lastModified }}</span>
            @endif
        </div>
    </div>

    {{-- Log content --}}
    <div class="rounded-xl border border-gray-200 dark:border-white/10 overflow-hidden">
        @if($this->errorMessage)
            <div class="p-4 text-sm text-gray-500 dark:text-gray-400 italic">
                {{ $this->errorMessage }}
            </div>
        @elseif(empty($this->logContent))
            <div class="p-4 text-sm text-gray-500 dark:text-gray-400 italic">
                {{ __('Log is empty.') }}
            </div>
        @else
            <div class="overflow-x-auto" style="max-height: 70vh">
                <pre class="bg-gray-900 p-4 text-xs leading-5 text-gray-200 font-mono whitespace-pre-wrap break-words">{{ $this->logContent }}</pre>
            </div>
        @endif
    </div>

    <x-filament-actions::modals />
</x-filament-panels::page>
