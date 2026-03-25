@props(['target' => 'activeTab'])

<div wire:loading wire:target="{{ $target }}" class="py-8 space-y-3">
    <div class="h-4 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
    <div class="h-4 w-full animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
    <div class="h-4 w-2/3 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
    <div class="h-4 w-5/6 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
</div>
