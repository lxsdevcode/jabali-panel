<div class="space-y-3">
    @if(!empty($this->selectedPaths))
        <div>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ __('Files/Folders:') }}</span>
            <div class="flex flex-wrap gap-1 mt-1">
                @foreach($this->selectedPaths as $path)
                    <x-filament::badge color="info">{{ basename($path) }}</x-filament::badge>
                @endforeach
            </div>
        </div>
    @endif

    @if(!empty($this->selectedDatabases))
        <div>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ __('Databases:') }}</span>
            <div class="flex flex-wrap gap-1 mt-1">
                @foreach($this->selectedDatabases as $db)
                    <x-filament::badge color="warning">{{ $db }}</x-filament::badge>
                @endforeach
            </div>
        </div>
    @endif

    @if(!empty($this->selectedMailboxes))
        <div>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ __('Mailboxes:') }}</span>
            <div class="flex flex-wrap gap-1 mt-1">
                @foreach($this->selectedMailboxes as $m)
                    <x-filament::badge color="gray">{{ $m }}</x-filament::badge>
                @endforeach
            </div>
        </div>
    @endif
</div>
