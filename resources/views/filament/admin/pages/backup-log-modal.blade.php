<div class="space-y-4">
    {{-- Backup info --}}
    <div class="grid grid-cols-2 gap-3 text-sm">
        <div>
            <span class="text-gray-500 dark:text-gray-400">{{ __('Status') }}:</span>
            <x-filament::badge class="ml-1" :color="match($backup->status) { 'completed' => 'success', 'failed' => 'danger', 'running' => 'warning', default => 'gray' }">
                {{ ucfirst($backup->status) }}
            </x-filament::badge>
        </div>
        <div>
            <span class="text-gray-500 dark:text-gray-400">{{ __('Destination') }}:</span>
            <span class="text-gray-950 dark:text-white ml-1">{{ $backup->destination?->name ?? __('Local') }}</span>
        </div>
        @if($backup->started_at)
            <div>
                <span class="text-gray-500 dark:text-gray-400">{{ __('Started') }}:</span>
                <span class="text-gray-950 dark:text-white ml-1">{{ $backup->started_at->format('M j, Y H:i:s') }}</span>
            </div>
        @endif
        @if($backup->completed_at)
            <div>
                <span class="text-gray-500 dark:text-gray-400">{{ __('Finished') }}:</span>
                <span class="text-gray-950 dark:text-white ml-1">{{ $backup->completed_at->format('M j, Y H:i:s') }}</span>
            </div>
        @endif
        @if($backup->duration)
            <div>
                <span class="text-gray-500 dark:text-gray-400">{{ __('Duration') }}:</span>
                <span class="text-gray-950 dark:text-white ml-1">{{ $backup->duration }}</span>
            </div>
        @endif
        @if($backup->size_bytes > 0)
            <div>
                <span class="text-gray-500 dark:text-gray-400">{{ __('Size') }}:</span>
                <span class="text-gray-950 dark:text-white ml-1">{{ $backup->size_human }}</span>
            </div>
        @endif
    </div>

    {{-- Error --}}
    @if($backup->error_message)
        <div class="rounded-lg bg-danger-50 p-3 text-sm text-danger-700 dark:bg-danger-400/10 dark:text-danger-400">
            {{ $backup->error_message }}
        </div>
    @endif

    {{-- Included items --}}
    <div class="flex flex-wrap gap-2">
        @if(!empty($backup->users))
            @foreach($backup->users as $u)
                <x-filament::badge color="gray">{{ $u }}</x-filament::badge>
            @endforeach
        @endif
        @if(!empty($backup->domains))
            @foreach($backup->domains as $d)
                <x-filament::badge color="info">{{ $d }}</x-filament::badge>
            @endforeach
        @endif
    </div>

    {{-- Restore history --}}
    @if($restores->isNotEmpty())
        <div class="border-t border-gray-200 dark:border-white/10 pt-3">
            <h4 class="text-sm font-medium text-gray-950 dark:text-white mb-2">{{ __('Restore History') }}</h4>
            <div class="space-y-2">
                @foreach($restores as $restore)
                    <div class="flex items-center gap-3 text-sm rounded-lg bg-gray-50 dark:bg-white/5 p-2">
                        <x-filament::badge :color="match($restore->status) { 'completed' => 'success', 'failed' => 'danger', 'running' => 'warning', default => 'gray' }">
                            {{ ucfirst($restore->status) }}
                        </x-filament::badge>
                        <span class="text-gray-600 dark:text-gray-300">{{ $restore->user?->username ?? '-' }}</span>
                        <span class="text-gray-500 dark:text-gray-400">{{ $restore->created_at->format('M j H:i') }}</span>
                        @if($restore->error_message)
                            <span class="text-danger-600 dark:text-danger-400 truncate">{{ $restore->error_message }}</span>
                        @endif
                    </div>
                @endforeach
            </div>
        </div>
    @endif
</div>
