<div class="space-y-3">
    @if($status)
        <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Status') }}:</span>
            <x-filament::badge :color="match($status) { 'completed' => 'success', 'failed' => 'danger', 'running' => 'warning', default => 'gray' }">
                {{ ucfirst($status) }}
            </x-filament::badge>
        </div>
    @endif

    @if($error)
        <div class="rounded-lg bg-danger-50 p-3 text-sm text-danger-700 dark:bg-danger-400/10 dark:text-danger-400">
            {{ $error }}
        </div>
    @endif

    @if(!empty($result))
        <div class="flex flex-wrap gap-3 text-sm">
            @if(isset($result['files_count']))
                <span class="text-gray-600 dark:text-gray-300">{{ __('Files') }}: {{ $result['files_count'] }}</span>
            @endif
            @if(isset($result['databases_count']))
                <span class="text-gray-600 dark:text-gray-300">{{ __('Databases') }}: {{ $result['databases_count'] }}</span>
            @endif
            @if(isset($result['mailboxes_count']))
                <span class="text-gray-600 dark:text-gray-300">{{ __('Mailboxes') }}: {{ $result['mailboxes_count'] }}</span>
            @endif
        </div>
    @endif

    @if($log)
        <div class="overflow-x-auto rounded-lg" style="max-height: 400px">
            <pre class="bg-gray-900 p-4 text-xs leading-5 text-gray-200 font-mono whitespace-pre-wrap break-words">{{ $log }}</pre>
        </div>
    @else
        <p class="text-sm text-gray-500 dark:text-gray-400 italic">{{ __('No log output available.') }}</p>
    @endif
</div>
